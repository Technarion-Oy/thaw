import re

with open("internal/snowflake/client.go", "r") as f:
    lines = f.readlines()

for i in range(len(lines)):
    line = lines[i]
    m = re.match(r"^(\s*)var result \[\]([A-Za-z0-9_*]+)\s*$", line)
    if m:
        indent = m.group(1)
        typ = m.group(2)
        lines[i] = f"{indent}result := []{typ}{{}}\n"

with open("internal/snowflake/client.go", "w") as f:
    f.writelines(lines)
