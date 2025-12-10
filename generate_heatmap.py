#!/usr/bin/env python3
"""
Generate a color-coded directory tree showing call graph distance from KesselAuthz.
Hottest = direct calls to KesselAuthz, cooler = further up the call stack.
"""

import os
import subprocess
from pathlib import Path
from collections import defaultdict

# Define heat levels (0 = hottest, higher = cooler)
# Level 0: KesselAuthz implementation itself
# Level 1: Direct callers of Authorizer interface
# Level 2: Callers of Level 1
# Level 3: Callers of Level 2
# etc.

HEAT_LEVELS = {
    # Level 0 - KesselAuthz itself
    "internal/authz/kessel/kessel.go": 0,
    
    # Level 1 - Direct callers of Authorizer methods
    "internal/biz/usecase/resources/resource_service.go": 1,
    "internal/consumer/consumer.go": 1,
    "internal/data/health/healthrepository.go": 1,
    
    # Level 2 - Callers of Level 1
    "internal/service/resources/kesselinventoryservice.go": 2,
    "internal/service/resources/kesselcheckservice.go": 2,
    "internal/biz/health/health.go": 2,
    
    # Level 3 - Callers of Level 2
    "internal/service/health/health.go": 3,
    "cmd/serve/serve.go": 3,
    "api/kessel/inventory/v1beta1/authz/check_grpc.pb.go": 3,
    "api/kessel/inventory/v1beta1/authz/check_http.pb.go": 3,
    "api/kessel/inventory/v1beta2/inventory_service_grpc.pb.go": 3,
    "api/kessel/inventory/v1beta2/inventory_service_http.pb.go": 3,
    "api/kessel/inventory/v1/health_grpc.pb.go": 3,
    "api/kessel/inventory/v1/health_http.pb.go": 3,
    
    # Level 4 - Callers of Level 3
    "cmd/root.go": 4,
    
    # Level 5 - Entry point
    "main.go": 5,
}

# Map files to the file they call at the next level down and the functions called
CALLS_DOWN = {
    # Level 1 files call Level 0
    "internal/biz/usecase/resources/resource_service.go": {
        "file": "internal/authz/kessel/kessel.go",
        "functions": ["Check", "CheckForUpdate", "LookupResources"]
    },
    "internal/consumer/consumer.go": {
        "file": "internal/authz/kessel/kessel.go",
        "functions": ["CreateTuples", "DeleteTuples", "Check", "AcquireLock"]
    },
    "internal/data/health/healthrepository.go": {
        "file": "internal/authz/kessel/kessel.go",
        "functions": ["Health"]
    },
    
    # Level 2 files call Level 1
    "internal/service/resources/kesselinventoryservice.go": {
        "file": "internal/biz/usecase/resources/resource_service.go",
        "functions": ["Check", "CheckForUpdate", "LookupResources"]
    },
    "internal/service/resources/kesselcheckservice.go": {
        "file": "internal/biz/usecase/resources/resource_service.go",
        "functions": ["CheckLegacy", "CheckForUpdateLegacy"]
    },
    "internal/biz/health/health.go": {
        "file": "internal/data/health/healthrepository.go",
        "functions": ["IsBackendAvailable"]
    },
    
    # Level 3 files call Level 2
    "internal/service/health/health.go": {
        "file": "internal/biz/health/health.go",
        "functions": ["IsBackendAvailable"]
    },
    "cmd/serve/serve.go": {
        "file": "internal/service/resources/kesselinventoryservice.go",
        "functions": ["New"]  # Creates/wires services
    },
    "api/kessel/inventory/v1beta1/authz/check_grpc.pb.go": {
        "file": "internal/service/resources/kesselcheckservice.go",
        "functions": ["Check", "CheckForUpdate"]  # gRPC handlers call service methods
    },
    "api/kessel/inventory/v1beta1/authz/check_http.pb.go": {
        "file": "internal/service/resources/kesselcheckservice.go",
        "functions": ["Check", "CheckForUpdate"]  # HTTP handlers call service methods
    },
    "api/kessel/inventory/v1beta2/inventory_service_grpc.pb.go": {
        "file": "internal/service/resources/kesselinventoryservice.go",
        "functions": ["Check", "CheckForUpdate", "LookupResources"]  # gRPC handlers call service methods
    },
    "api/kessel/inventory/v1beta2/inventory_service_http.pb.go": {
        "file": "internal/service/resources/kesselinventoryservice.go",
        "functions": ["Check", "CheckForUpdate", "LookupResources"]  # HTTP handlers call service methods
    },
    "api/kessel/inventory/v1/health_grpc.pb.go": {
        "file": "internal/service/health/health.go",
        "functions": ["GetReadyz"]  # gRPC handlers call health service methods
    },
    "api/kessel/inventory/v1/health_http.pb.go": {
        "file": "internal/service/health/health.go",
        "functions": ["GetReadyz"]  # HTTP handlers call health service methods
    },
    
    # Level 4 files call Level 3
    "cmd/root.go": {
        "file": "cmd/serve/serve.go",
        "functions": ["NewCommand"]
    },
    
    # Level 5 files call Level 4
    "main.go": {
        "file": "cmd/root.go",
        "functions": ["Execute"]
    },
}

# ANSI color codes for heatmap (red = hot, blue = cool, white = no connection)
COLORS = {
    0: "\033[38;5;196m",  # Bright red
    1: "\033[38;5;208m",  # Orange-red
    2: "\033[38;5;220m",  # Yellow
    3: "\033[38;5;226m",  # Bright yellow
    4: "\033[38;5;46m",   # Green
    5: "\033[38;5;51m",   # Cyan
    6: "\033[38;5;27m",   # Blue
    7: "\033[38;5;21m",   # Dark blue
    None: "\033[0m",       # White (no connection)
}
RESET = "\033[0m"

def get_file_heat_level(filepath):
    """Get heat level for a file."""
    # Normalize path
    normalized = filepath.lstrip('./')
    return HEAT_LEVELS.get(normalized)

def get_called_file(filepath):
    """Get the file that this file calls at the next level down."""
    # Normalize path
    normalized = filepath.lstrip('./')
    call_info = CALLS_DOWN.get(normalized)
    if isinstance(call_info, dict):
        return call_info.get("file")
    return call_info

def get_called_functions(filepath):
    """Get the functions that this file calls at the next level down."""
    # Normalize path
    normalized = filepath.lstrip('./')
    call_info = CALLS_DOWN.get(normalized)
    if isinstance(call_info, dict):
        return call_info.get("functions", [])
    return []

def format_functions(functions):
    """Format a list of function names as a comma-separated string."""
    if not functions:
        return ""
    
    return ", ".join(functions)

def build_directory_tree(root_dir):
    """Build a directory tree structure."""
    tree = {}
    root_path = Path(root_dir)
    
    for go_file in root_path.rglob("*.go"):
        # Skip vendor, .git, etc.
        if "vendor" in go_file.parts or ".git" in go_file.parts:
            continue
            
        rel_path = go_file.relative_to(root_path)
        parts = rel_path.parts
        
        # Build nested dict structure
        current = tree
        for part in parts[:-1]:
            if part not in current:
                current[part] = {}
            current = current[part]
        
        # Add file with heat level and called file info
        filename = parts[-1]
        if not isinstance(current, dict):
            current = {}
        heat_level = get_file_heat_level(str(rel_path))
        called_file = get_called_file(str(rel_path))
        called_functions = get_called_functions(str(rel_path))
        current[filename] = {
            'heat': heat_level,
            'called_file': called_file,
            'called_functions': called_functions
        }
    
    return tree

def has_relevant_files(node):
    """Check if a directory node contains any files with heat levels (in call stack)."""
    if not isinstance(node, dict):
        return False
    
    for name, child in node.items():
        if isinstance(child, dict):
            if 'heat' in child:
                # This is a file with metadata
                heat = child.get('heat')
                if heat is not None:
                    return True
            else:
                # This is a directory, check recursively
                if has_relevant_files(child):
                    return True
    return False

def get_sort_key(name, child):
    """Get sort key for items: heat level (descending, so cool to hot), then type, then name."""
    if isinstance(child, dict):
        if 'heat' in child:
            # File with metadata
            heat = child.get('heat')
            if heat is not None:
                # Sort by heat level descending (cool to hot: 5, 4, 3, 2, 1, 0)
                # Use negative heat so higher numbers come first
                return (-heat, 1, name)  # 1 = file
            else:
                return (999, 1, name)  # Files without heat go last
        else:
            # Directory - get the minimum heat level from children (most relevant)
            min_heat = get_min_heat_in_subtree(child)
            if min_heat is not None:
                return (-min_heat, 0, name)  # 0 = directory
            else:
                return (999, 0, name)  # Directories without heat go last
    return (999, 1, name)

def get_min_heat_in_subtree(node):
    """Get the minimum (hottest) heat level in a subtree."""
    if not isinstance(node, dict):
        return None
    
    min_heat = None
    for child in node.values():
        if isinstance(child, dict):
            if 'heat' in child:
                heat = child.get('heat')
                if heat is not None:
                    if min_heat is None or heat < min_heat:
                        min_heat = heat
            else:
                # Directory - recurse
                sub_heat = get_min_heat_in_subtree(child)
                if sub_heat is not None:
                    if min_heat is None or sub_heat < min_heat:
                        min_heat = sub_heat
    return min_heat

def print_tree(node, prefix="", is_last=True, max_depth=10, depth=0):
    """Print directory tree with color coding, skipping directories with no relevant files."""
    if depth > max_depth:
        return
    
    if isinstance(node, dict):
        # Filter items to only show directories/files with relevant content
        items = []
        for name, child in node.items():
            if isinstance(child, dict):
                if 'heat' in child:
                    # File with metadata - include if it has a heat level
                    if child.get('heat') is not None:
                        items.append((name, child))
                else:
                    # Directory - include if it has relevant files
                    if has_relevant_files(child):
                        items.append((name, child))
        
        # Sort: by heat level (cool to hot: 5,4,3,2,1,0), then directories before files, then alphabetically
        items = sorted(items, key=lambda x: get_sort_key(x[0], x[1]))
        
        for i, (name, child) in enumerate(items):
            is_last_item = (i == len(items) - 1)
            current_prefix = "└── " if is_last_item else "├── "
            next_prefix = prefix + ("    " if is_last_item else "│   ")
            
            # Determine if this is a directory or file
            if isinstance(child, dict):
                # Check if it's a file metadata dict or a directory
                if 'heat' in child:
                    # File with metadata
                    heat = child['heat']
                    called_file = child.get('called_file')
                    called_functions = child.get('called_functions', [])
                    color = COLORS.get(heat, COLORS[None])
                    heat_str = f"[{heat}]" if heat is not None else "[ ]"
                    
                    # Show called file and functions for levels > 0
                    if heat is not None and heat > 0 and called_file:
                        # Extract just the filename from the called file path
                        called_filename = Path(called_file).name
                        called_color = COLORS.get(heat-1, COLORS[None])
                        
                        # Add function names if available
                        if called_functions:
                            func_str = format_functions(called_functions)
                            called_str = f" → {called_color}{called_filename}{RESET}({func_str})"
                        else:
                            called_str = f" → {called_color}{called_filename}{RESET}"
                    else:
                        called_str = ""
                    
                    print(f"{prefix}{current_prefix}{color}{name}{RESET} {heat_str}{called_str}")
                else:
                    # Directory - only print if it has relevant files
                    if has_relevant_files(child):
                        print(f"{prefix}{current_prefix}{name}/")
                        print_tree(child, next_prefix, is_last_item, max_depth, depth + 1)
            else:
                # Legacy: file with just heat level (shouldn't happen with new structure)
                heat = child
                if heat is not None:
                    color = COLORS.get(heat, COLORS[None])
                    heat_str = f"[{heat}]"
                    print(f"{prefix}{current_prefix}{color}{name}{RESET} {heat_str}")

def main():
    root_dir = Path(__file__).parent
    tree = build_directory_tree(root_dir)
    
    print("=" * 80)
    print("HEATMAP LEGEND:")
    print("=" * 80)
    print(f"{COLORS[0]}Level 0 (Hottest): KesselAuthz implementation{RESET}")
    print(f"{COLORS[1]}Level 1: Direct callers of Authorizer interface{RESET}")
    print(f"{COLORS[2]}Level 2: Callers of Level 1{RESET}")
    print(f"{COLORS[3]}Level 3: Callers of Level 2{RESET}")
    print(f"{COLORS[4]}Level 4: Callers of Level 3{RESET}")
    print(f"{COLORS[5]}Level 5: Entry point{RESET}")
    print(f"{COLORS[None]}White: No connection to KesselAuthz call stack{RESET}")
    print("=" * 80)
    print()
    print("DIRECTORY TREE (heatmap by call distance from KesselAuthz):")
    print("=" * 80)
    print()
    
    print_tree(tree)

if __name__ == "__main__":
    main()

