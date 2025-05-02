import json
import re
import subprocess
import sys
import os


def extract_json_payload(line: str):
    if "operation:" in line:
        parts = line.strip().split('\t')
    
        # extract the operation
        op_match = re.search(r'operation:(\w+)', parts[1])
        if not op_match:
            return None
        operation = op_match.group(1)
        print("operation " + operation)

        # extract inventory_id from payload
        payload1_json = json.loads(parts[2])
        inventory_id = payload1_json.get("payload")

        # get the other payload with the permission data.
        data = json.loads(parts[-1])

        return inventory_id, data.get("payload"), operation
    else:
        match = re.search(r'"payload":"({.+})"', line)
        payload_str = match.group(1).encode().decode("unicode_escape")
        return None, json.loads(payload_str), "updated"


def build_zed_command(inventory_id: str, payload: any, operation: str):
    if operation == "deleted":
        # parse message
        resource_namespace = payload["resource_namespace"]
        resource_name = payload["resource_type"]
        resource_id = payload["resource_id"]
        relation = payload["relation"]
        # 1
        # If the resource does not exist in inventory, check if tuple exists, if so delete the tuple
		# (this is in the case of a delete operation message & no resource in inventory)
		# (parse message, gabi call, zed relationship read, zed relationship delete) 
        # call gabi

        # Call gabi to fetch current info on resource.
        inventory_resource_id, inventory_subject_id = fetch_inventory_resource_info(inventory_id)

        # resource doesn't exist in inventory
        print("Resource doesn't exist in Inventory DB.")

        # zed relationship read fully consistent
        res = subprocess.run(
                f"zed relationship read "
                f"{resource_namespace}/{resource_name}:{resource_id} "
                f"t_{relation} --consistency-full",
                shell=True,
                capture_output=True,
                text=True
            )
        
        # exists.
        if len(res.stdout) != 0:
            print(f"Resource exists in SpiceDB: {res.stdout}")

            return (
                f"zed relationship bulk-delete "
                f"{resource_namespace}/{resource_name}:{resource_id} "
                f"t_{relation} "
            )
        else:
            return None
        
    else: # created or updated
        # parse message
        resource = payload["resource"]
        relation = payload["relation"]
        subject = payload["subject"]["subject"]

        resource_namespace = resource["type"]["namespace"]
        if resource_namespace == "authz":
            resource_namespace = "notifications"

        resource_name = resource["type"]["name"]
        resource_id = resource["id"]

        subject_ns = subject["type"]["namespace"]
        subject_name = subject["type"]["name"]
        subject_id = subject["id"]
    
        if operation == "created":
            # 	2) If the resource exists in inventory, check if tuple exists, if not create
            # (in the case of resource in inventory but not in spicedb.)
            # (parse message, gabi call, zed relationship read, zed relationship create)

            # Call gabi to fetch current info on resource.
            inventory_resource_id, inventory_subject_id = fetch_inventory_resource_info(inventory_id)

            # resource exists
            print("Resource exists in Inventory DB.")

            # check if tuple already exists!

            # zed relationship read fully consistent
            res = subprocess.run(
                    f"zed relationship read "
                    f"{resource_namespace}/{resource_name}:{resource_id} "
                    f"t_{relation} "
                    f"{subject_ns}/{subject_name}:{subject_id} --consistency-full",
                    shell=True,
                    capture_output=True,
                    text=True
                )
            
            # exists.
            if len(res.stdout) != 0:
                print(f"Resource exists in SpiceDB: {res.stdout}")
                return None

            # Only create if exists in inventory & doesn't exist in SpiceDB.
            return (
                f"zed relationship create "
                f"{resource_namespace}/{resource_name}:{resource_id} "
                f"t_{relation} "
                f"{subject_ns}/{subject_name}:{subject_id}"
            )
    
        elif operation == "updated":
            # 4) if the resource exists in inventory, check if the tuple exists, if they do not match, apply the update to the tuple derived from the resource
			# (so then this is an update okay,)
			# (what about them needs to match? this is were im a bit confused...)
			# (parse mesasge, gabi call, zed relatonship read, zed relationship touch)

            # Call gabi to fetch current info on resource.
            inventory_resource_id, inventory_subject_id = fetch_inventory_resource_info(inventory_id)

            # zed relationship read fully consistent
            res = subprocess.run(
                    f"zed relationship read "
                    f"{resource_namespace}/{resource_name}:{resource_id} "
                    f"t_{relation} "
                    f"{subject_ns}/{subject_name}:{subject_id} --consistency-full",
                    shell=True,
                    capture_output=True,
                    text=True
                )
            zed_output = res.stdout
            
            # exists.
            if len(zed_output) != 0:
                print(f"Resource exists in SpiceDB: {zed_output}")
                # do they match?
                # If not, apply update from inventory DB.
                sections = zed_output.split()
                res_id = sections[0].split(":")[1]
                sub_id = sections[2].split(":")[1]

                if res_id == inventory_resource_id and sub_id == inventory_subject_id:
                    return None # They match, no need to update.

                return (
                    f"zed relationship touch "
                    f"{resource_namespace}/{resource_name}:{inventory_resource_id} "
                    f"t_{relation} "
                    f"{subject_ns}/{subject_name}:{inventory_subject_id}"
                )
            
            # If it doesn't exist in spicedb but does in inventory, we should prob create?
            return (
                f"zed relationship touch "
                f"{resource_namespace}/{resource_name}:{inventory_resource_id} "
                f"t_{relation} "
                f"{subject_ns}/{subject_name}:{inventory_subject_id}"
            )

def fetch_inventory_resource_info(inventory_id):
    # Call gabi to fetch current info on resource.
    print(f"Fetching info on inventory_id: {inventory_id}")
    res = subprocess.run(
        f"gabi exec \"select * from resources where inventory_id='{inventory_id}'\"",
        shell=True,
        capture_output=True,
        text=True
    )
    gabi_output = res.stdout
    if gabi_output == "your query didn't return any results":
        return None # Don't update anything.

    # resource exists
    print("Resource exists in Inventory DB.")
    parsed_data = json.loads(gabi_output)
    
    inventory_resource_id = parsed_data[0]["reporter_resource_id"]
    inventory_subject_id = parsed_data[0]["workspace_id"]

    print(f"Inventory_resource_id: {inventory_resource_id}")
    print(f"Inventory_subject_id: {inventory_subject_id}")
    return inventory_resource_id, inventory_subject_id


def main():
    if len(sys.argv) != 2:
        print("Usage: python3 manual_migration_to_spicedb.py <input_file>")
        sys.exit(1)

    input_file = sys.argv[1]

    if not os.path.isfile(input_file):
        print(f"File not found: {input_file}")
        return

    with open(input_file, "r") as f:
        for line in f:
            inventory_id, payload, operation = extract_json_payload(line)
            if not payload:
                continue

            cmd = build_zed_command(inventory_id, payload, operation)
            if cmd:
                print(f"Running: {cmd}")
                try:
                    subprocess.run(cmd, shell=True, check=True)
                except subprocess.CalledProcessError as e:
                    print(f"Command failed with return code {e.returncode}")
                    sys.exit(e.returncode)


if __name__ == "__main__":
    main()
