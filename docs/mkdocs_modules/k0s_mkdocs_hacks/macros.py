import os
import re
import subprocess

def define_env(env):
    @env.filter
    def ljust(value, width):
        return str(value).ljust(width)

    # Export the Go literals as a macro. Don't do it as a variable, as
    # non-existent variables won't fail the docs build and would go unnoticed.
    # The macro will fail with a KeyError if there's no such literal.
    src_vars = parse_src_string_literals()
    @env.macro
    def src_var(name):
        return src_vars[name]

    build_vars = {}
    @env.macro
    def build_var(name):
        if name not in build_vars:
            proc = subprocess.run(['./vars.sh', name], capture_output=True, check=True, text=True)
            build_vars[name] = proc.stdout.strip()
        return build_vars[name]

    env.variables.k0s_version = os.environ['K0S_VERSION']
    env.variables.k0s_docker_version = os.environ['K0S_VERSION'].replace('+', '-')
    env.variables.k8s_version = 'v' + build_var('kubernetes_version')

def parse_src_string_literals():
    # Hand-wavy pattern to get all literal consts/vars from a go file.
    pattern = r'^(?:const|var)?\s+(\w+)\s*=\s*"([^"]+)"'
    return parse_file_kv('pkg/constant/constant.go', pattern)

def parse_file_kv(file_path, pattern):
    pattern = re.compile(pattern)
    parsed = {}
    with open(file_path, 'r') as file:
        for line_number, line in enumerate(file, start=1):
            match = pattern.search(line)
            if match:
                parsed[match.group(1)] = match.group(2)
    return parsed
