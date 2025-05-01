import json
import re
import subprocess
import sys
import os


def extract_json_payload(line):
    if "operation:" in line:
        # ignore delete for now.
        if "operation:deleted" in line:
            return None

        # its tab seperated??
        parts = line.strip().split('\t')

        data = json.loads(parts[-1])
        return data.get("payload")
    else:
        match = re.search(r'"payload":"({.+})"', line)
        payload_str = match.group(1).encode().decode("unicode_escape")
        return json.loads(payload_str)


def build_zed_command(payload):
    resource = payload["resource"]
    relation = payload["relation"]
    subject = payload["subject"]["subject"]

    resource_name = resource["type"]["name"]
    resource_id = resource["id"]

    subject_ns = subject["type"]["namespace"]
    subject_name = subject["type"]["name"]
    subject_id = subject["id"]

    return (
        f"zed relationship touch "
        f"notifications/{resource_name}:{resource_id} "
        f"t_{relation} "
        f"{subject_ns}/{subject_name}:{subject_id}"
    )


def main():
    if len(sys.argv) != 2:
        print("Usage: python3 migrate_to_spicedb.py <input_file>")
        sys.exit(1)

    input_file = sys.argv[1]

    if not os.path.isfile(input_file):
        print(f"File not found: {input_file}")
        return

    with open(input_file, "r") as f:
        for line in f:
            payload = extract_json_payload(line)
            if not payload:
                continue

            cmd = build_zed_command(payload)
            print(f"Running: {cmd}")
            subprocess.run(cmd, shell=True)

if __name__ == "__main__":
    main()
