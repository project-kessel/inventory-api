import json
import re
import subprocess
import sys
import os


def extract_json_payload(line):
    if "operation:" in line:
        # its tab seperated??
        parts = line.strip().split('\t')

        data = json.loads(parts[-1])
        if "operation:deleted" in line: # delete 
            return data.get("payload"), True

        return data.get("payload"), False
    else:
        match = re.search(r'"payload":"({.+})"', line)
        payload_str = match.group(1).encode().decode("unicode_escape")
        return json.loads(payload_str), False


def build_zed_command(payload, delete: bool):
    if delete:
        resource_namespace = payload["resource_namespace"]
        resource_name = payload["resource_type"]
        resource_id = payload["resource_id"]
        relation = payload["relation"]

        return (
            f"zed relationship bulk-delete "
            f"{resource_namespace}/{resource_name}:{resource_id} "
            f"t_{relation} "
        )
    else: # upsert
    
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

        return (
            f"zed relationship touch "
            f"{resource_namespace}/{resource_name}:{resource_id} "
            f"t_{relation} "
            f"{subject_ns}/{subject_name}:{subject_id}"
        )


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
            payload, delete = extract_json_payload(line)
            if not payload:
                continue

            cmd = build_zed_command(payload, delete)
            print(f"Running: {cmd}")
            try:
                subprocess.run(cmd, shell=True, check=True)
            except subprocess.CalledProcessError as e:
                print(f"Command failed with return code {e.returncode}")
                sys.exit(e.returncode)

if __name__ == "__main__":
    main()
