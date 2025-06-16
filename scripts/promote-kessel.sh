#!/bin/bash


# !!! This script must be run at the root of a local clone of App Interface to work !!!
# It's recommended you copy this script to your PATH so it can be
# used locally while in App Interface Path

AVAILABLE_SERVICES=("relations-api" "inventory-api" "relations-sink-connector" "inventory-api-debezium-connector" "spicedb-operator")


help_me() {
    echo "Promote Kessel: A basic script for updating the commit hash for"
    echo "production Kessel deployments in App Interface"
    echo "NOTE: This must be run in the root of a local clone of App Interface!"
    echo ""
    echo "Usage: promote.sh -s <service name> -g [git hash]"
    echo "   -s service name: name of service (see available service names below)"
    echo "   -g git hash: target git commit in the service, if not specified defaults to HEAD of master/main"
    echo ""
    echo "Examples:"
    echo "Promote Inventory to latest:"
    echo "  promote.sh -s inventory-api"
    echo ""
    echo "Promote Relations to a specific commit hash:"
    echo "  promote.sh -s relations-api -g abc123"

    echo -e "\nAvailable service names:"
    printf "%s\n" "${AVAILABLE_SERVICES[@]}"
    echo ""
}

backout() {
    echo "backing out..."
    git checkout master
    git branch -D "promote-$SERVICE_NAME-$GIT_HASH"
}

validate() {

  if [ $IN_APPINTERFACE -eq 0 ]; then
      echo "ERROR: Not running in checkout of app-interface."
      echo "Please run script in checkout of https://gitlab.cee.redhat.com/service/app-interface"
      exit 3
  fi

  if [ "$SERVICE_NAME" = "" ];then
    help_me
    exit 2
  fi

  # is this a valid service name?
  if [[ ! ${AVAILABLE_SERVICES[@]} =~ $SERVICE_NAME ]]; then
    echo "Invalid Service Name: $SERVICE_NAME"
    echo -e "\nAvailable service names:"
    printf "%s\n" "${AVAILABLE_SERVICES[@]}"
    echo ""
    exit 5
  fi

  # must not be behind upstream master (saas-osd-operators repo)
  BEHIND_COUNT=$(git rev-list HEAD..${REMOTE}/master --count)

  if [ $BEHIND_COUNT -ne 0 ]; then
      echo "FAILURE: you are behind 'master' by this many commits: $BEHIND_COUNT"
      exit 4
  fi

  # check if the user has yq installed otherwise the tool doesnt work
  if ! command -v yq &>/dev/null; then
    echo "ERROR: yq cli is required to update saas files -- please install"
    echo "https://github.com/mikefarah/yq#install"
    exit 1
  fi
}

update_saas_file() {
  case $SERVICE_NAME in
      spicedb-operator)
        DEPLOY_FILE="deploy-spicedb-operator.yml"
        export NAMESPACE="crcp"
        ;;
      relations-sink-connector | inventory-api-debezium-connector)
        DEPLOY_FILE="deploy.yml"
        export NAMESPACE="platform-mq-prod"
        ;;
      *)
        DEPLOY_FILE="deploy.yml"
        export NAMESPACE="kessel-prod"
        ;;
  esac


  # get remote url
  if ! python3 -c 'import yaml'; then
  	echo "FAILURE: the yaml module was not present, please install it to continue"
  	exit 5
  fi

  SERVICE_JSON=$(cat $BASE_DIR/$KESSEL_DIR/$DEPLOY_FILE | python3 -c 'import json, sys, yaml ; y=yaml.safe_load(sys.stdin.read()) ; json.dump(y, sys.stdout)')

  CURRENT_GIT_HASH=$(echo $SERVICE_JSON | jq --arg namespace "$NAMESPACE" --arg service "${SERVICE_NAME}" -r '.resourceTemplates[] | select(.name == $service) | .targets[] | select(.namespace["$ref"] | contains($namespace)) | .ref' | uniq)


  if [ "$CURRENT_GIT_HASH" == "" ]; then
      echo "FAILURE: Unable to determine current git hash from SAAS file: $BASE_DIR/$KESSEL_DIR/$DEPLOY_FILE"
      exit 6
  fi

  GIT_URL=$(echo $SERVICE_JSON | jq --arg service "${SERVICE_NAME}" -r '.resourceTemplates[] | select(.name == $service) | .url')

  # check it out and get the latest hash (if wasn't passed in) and log messages
  TEMP_DIR=$(mktemp -d)

  pushd $TEMP_DIR 2>/dev/null

  # clone the repo
  git clone "$GIT_URL" source-dir

  pushd source-dir 2>/dev/null

  if [ "$GIT_HASH" = "" ]; then
      # didn't get a git hash, find latest
      export GIT_HASH=$(git rev-parse HEAD)
  fi

  echo if [ "$CURRENT_GIT_HASH" == "$GIT_HASH" ];

  if [ "$CURRENT_GIT_HASH" == "$GIT_HASH" ]; then
      echo -e "\nNOTHING TO PROMOTE, $SERVICE_NAME is at target hash: $GIT_HASH"
      exit -1
  fi

  # get commit subject and message for the promotion
  echo "Promote $SERVICE_NAME $GIT_HASH" > ../message.log
  echo "" >> ../message.log
  echo "${GIT_URL}/compare/${CURRENT_GIT_HASH}...${GIT_HASH}" >> ../message.log
  echo '
  ---
  ```' >> ../message.log
  git log --no-merges --pretty=format:'commit: %H%nauthor: %an%n%s%n%n%b%n%n' "$CURRENT_GIT_HASH".."$GIT_HASH" >> ../message.log
  echo '```' >> ../message.log

  popd 2>/dev/null
  popd 2>/dev/null

  # make the change (update hash)
  yq e -i '(.resourceTemplates[].targets[] | select(.namespace."$ref" | contains(env(NAMESPACE)))).ref |= env(GIT_HASH)' $BASE_DIR/$KESSEL_DIR/$DEPLOY_FILE

  # create branch for promotion
  git checkout -b "promote-$SERVICE_NAME-$GIT_HASH" ${REMOTE}/master
  if [[ "$?" -ne 0 ]]; then backout && exit 6; fi

  # commit the change
  git add -u $BASE_DIR/$KESSEL_DIR/$DEPLOY_FILE
  if [[ "$?" -ne 0 ]]; then backout && exit 6; fi

  git commit -F "$TEMP_DIR/message.log"
  if [[ "$?" -ne 0 ]]; then backout && exit 6; fi
}

### Main ###

# Set prereq vars
IN_APPINTERFACE=$((git remote -v 2>&1 || echo "0") | grep "gitlab.cee.redhat.com" | grep "app-interface" | wc -l)
REMOTE=${REMOTE:-"upstream"}
BASE_DIR=$(git rev-parse --show-toplevel 2>/dev/null)
KESSEL_DIR="data/services/insights/kessel"

# Identify and force use of GNU sed
sed_help="$(LANG=C sed --help 2>&1 || true)"
if echo "${sed_help}" | grep -q "GNU\|BusyBox"; then
    SED="sed"
elif command -v gsed &>/dev/null; then
    SED="gsed"
else
    echo "Failed to find GNU sed as sed or gsed. If you are on Mac: brew install gnu-sed." >&2
    exit 1
fi

while getopts "s:g:h" flag; do
    case "${flag}" in
        s) SERVICE_NAME=${OPTARG};;
        g) export GIT_HASH=${OPTARG};;
        h) help_me; exit 0;;
    esac
done

# ensure we're on master branch
git checkout master &> /dev/null

validate

update_saas_file

echo ""
echo "service: $SERVICE_NAME"
echo "from: $CURRENT_GIT_HASH"
echo "to:   $GIT_HASH"
echo "READY TO PUSH, $SERVICE_NAME promotion commit is ready locally"
