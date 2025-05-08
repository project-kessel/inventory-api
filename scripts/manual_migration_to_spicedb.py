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
            return None, None, None
        operation = op_match.group(1)

        # extract inventory_id from payload
        payload1_json = json.loads(parts[2])
        inventory_id = payload1_json.get("payload")

        # get the other payload with the permission data.
        data = json.loads(parts[-1])

        return inventory_id, data.get("payload"), operation
    else:
        match = re.search(r'"payload":"({.+})"', line)
        if not match:
            return None, None, None
        payload_str = match.group(1).encode().decode("unicode_escape")

        # extract inventory_id from payload.
        match_inventory_id = re.search(r'"payload":"(.{36})"', line)
        if not match_inventory_id:
            return None, None, None
        inventory_id = match_inventory_id.group(1).encode().decode("unicode_escape")

        # either updated or deleted depending on if a subject is defined.
        operation = "updated"
        payload = json.loads(payload_str)
        if not payload.get("subject"):
            operation = "deleted"

        return inventory_id, payload, operation
    

def build_deleted_command(inventory_id, payload):
    # If the resource does not exist in inventory, check if tuple exists, if so delete the tuple
	# (parse message, gabi call, zed relationship read, zed relationship delete) 

    # parse message
    resource_namespace = payload["resource_namespace"]
    resource_name = payload["resource_type"]
    resource_id = payload["resource_id"]
    relation = payload["relation"]

    # Call gabi to fetch current info on resource.
    inv_res_id, inv_sub_id, inv_res_type, inv_report_type = fetch_inventory_resource_info(inventory_id)
    if all([inv_res_id, inv_sub_id, inv_res_type, inv_report_type]):
        return None # Dont delete, as it still exists in inventory DB.

    # zed relationship read fully consistent (drift check)
    res = subprocess.run(
        f"zed relationship read {resource_namespace}/{resource_name}:{resource_id} t_{relation} --consistency-full",
        shell=True, capture_output=True, text=True
    )
    if len(res.stdout) != 0:
        print(f"Resource exists in SpiceDB: {res.stdout}. Deleting...")
        return f"zed relationship bulk-delete {resource_namespace}/{resource_name}:{resource_id} t_{relation}"

    print("Resource doesn't exist in SpiceDB. Nothing to delete...")
    return None

def build_created_command(inventory_id, payload):
    # If the resource exists in inventory, check if tuple exists, if not create
    # (parse message, gabi call, zed relationship read, zed relationship create)

    # parse message
    relation = payload["relation"]
    subject = payload["subject"]["subject"]

    subject_ns = subject["type"]["namespace"]
    subject_name = subject["type"]["name"]

    # Call gabi to fetch current info on resource.
    inv_res_id, inv_sub_id, inv_res_type, inv_report_type = fetch_inventory_resource_info(inventory_id)
    if not all([inv_res_id, inv_sub_id, inv_res_type, inv_report_type]):
        return None # No resource in inventory DB.

    # zed relationship read fully consistent (drift check)
    res = subprocess.run(
        f"zed relationship read {inv_report_type}/{inv_res_type}:{inv_res_id} t_{relation} {subject_ns}/{subject_name}:{inv_sub_id} --consistency-full",
        shell=True, capture_output=True, text=True
    )
    if len(res.stdout) != 0:
        print(f"Resource exists in SpiceDB: {res.stdout}. Not creating.")
        return None # Our job here is already done.
    print("Resource exists in inventory, but not in SpiceDB. Creating...")
    return f"zed relationship create {inv_report_type}/{inv_res_type}:{inv_res_id} t_{relation} {subject_ns}/{subject_name}:{inv_sub_id}"

def build_updated_command(inventory_id, payload):
    # if the resource exists in inventory, check if the tuple exists, if they do not match,
    # apply the update to the tuple derived from the resource
    # (parse mesasge, gabi call, zed relatonship read, zed relationship touch)

    # parse message
    relation = payload["relation"]
    subject = payload["subject"]["subject"]

    subject_ns = subject["type"]["namespace"]
    subject_name = subject["type"]["name"]

    # Call gabi to fetch current info on resource.
    inv_res_id, inv_sub_id, inv_res_type, inv_report_type = fetch_inventory_resource_info(inventory_id)
    if not all([inv_res_id, inv_sub_id, inv_res_type, inv_report_type]):
        return None # No resource in inventory DB.

    # zed relationship read fully consistent (drift check)
    res = subprocess.run(
        f"zed relationship read {inv_report_type}/{inv_res_type}:{inv_res_id} t_{relation} {subject_ns}/{subject_name}:{inv_sub_id} --consistency-full",
        shell=True, capture_output=True, text=True
    )
    if len(res.stdout) != 0:
        print(f"Resource exists in SpiceDB: {res.stdout}")
        sections = res.stdout.split()
        if len(sections) < 3:
            print(f"Unexpected output format from zed relationship read: {res.stdout}")
            return None
        current_res_id = sections[0].split(":")[1]
        current_sub_id = sections[2].split(":")[1]
        if current_res_id == inv_res_id and current_sub_id == inv_sub_id:
            print("Resources match, no need to update.")
            return None # They match, no need to update.
        print("Updating resource...")
        return f"zed relationship touch {inv_report_type}/{inv_res_type}:{inv_res_id} t_{relation} {subject_ns}/{subject_name}:{inv_sub_id}"

    print("Resource doesn't exist in SpiceDB but does in inventory, creating...")
    return f"zed relationship touch {inv_report_type}/{inv_res_type}:{inv_res_id} t_{relation} {subject_ns}/{subject_name}:{inv_sub_id}"
    

def build_zed_command(inventory_id: str, payload: any, operation: str):
    print(f"\nBuilding command for Inventory ID: {inventory_id}, operation: {operation}")
    if operation == "deleted":
        return build_deleted_command(inventory_id, payload)
    elif operation == "created":
        return build_created_command(inventory_id, payload)
    elif operation == "updated":
        return build_updated_command(inventory_id, payload)
    else:
        print("Unsupported operation")
        return None

def fetch_inventory_resource_info(inventory_id):
    # Call gabi to fetch current info on resource.
    print(f"Fetching info on inventory_id: {inventory_id}")
    res = subprocess.run(
        f"gabi exec \"select reporter_resource_id, workspace_id, resource_type, reporter_type, reporter from resources where inventory_id='{inventory_id}'\"",
        shell=True,
        capture_output=True,
        text=True
    )
    gabi_output = res.stdout
    if "your query didn't return any results" in gabi_output: # there's some weird tabbing in output.
        print("Nothing in inventory db.")
        return None, None, None, None # Don't update anything.

    # resource exists
    print(f"Inventory ID: {inventory_id} exists in Inventory DB.")
    parsed_data = json.loads(gabi_output)
    if not parsed_data:
        return None, None, None, None
    
    inv_resource_id = parsed_data[0]["reporter_resource_id"]
    inv_subject_id = parsed_data[0]["workspace_id"]
    inv_resource_type = parsed_data[0]["resource_type"]

    inv_reporter_type = parsed_data[0]["reporter_type"].lower()
    parse_reporter = json.loads(parsed_data[0]["reporter"])
    inv_reporter_reporter_type = parse_reporter["reporter_type"].lower()

    inv_reporter_type = inv_reporter_type or inv_reporter_reporter_type

    print(f"Inventory_resource_id: {inv_resource_id}")
    print(f"Inventory_subject_id: {inv_subject_id}")
    print(f"Inventory_resource_type: {inv_resource_type}")
    print(f"Inventory_reporter_type: {inv_reporter_type}")
    return inv_resource_id, inv_subject_id, inv_resource_type, inv_reporter_type


def main():
    if len(sys.argv) != 2:
        print("Usage: python3 manual_migration_to_spicedb.py <input_file>")
        sys.exit(1)

    input_file = sys.argv[1]

    if not os.path.isfile(input_file):
        print(f"File not found: {input_file}")
        return
    
    dry_run = os.getenv("DRY_RUN", "false").lower() == "true"
    if dry_run:
        print("Dry run mode enabled.\n")

    with open(input_file, "r") as f:
        for line in f:
            inventory_id, payload, operation = extract_json_payload(line)
            if not payload:
                continue

            cmd = build_zed_command(inventory_id, payload, operation)
            if cmd:
                if dry_run:
                    print(f"Would update {inventory_id} with {cmd}")
                else:
                    print(f"Running: {cmd}")
                    try:
                        subprocess.run(cmd, shell=True, check=True)
                    except subprocess.CalledProcessError as e:
                        print(f"Command failed with return code {e.returncode}")
                        sys.exit(e.returncode)
            elif dry_run:
                print(f"Skipping {inventory_id}, no update needed")


if __name__ == "__main__":
    main()
