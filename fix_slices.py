import re

with open("internal/snowflake/client.go", "r") as f:
    lines = f.readlines()

for i in range(len(lines)):
    line = lines[i]
    # Match standard var slice declarations
    m = re.match(r"^(\s*)var (\w+) \[\]([A-Za-z0-9_*]+)\s*$", line)
    if m:
        indent = m.group(1)
        name = m.group(2)
        typ = m.group(3)
        
        # Skip 'der' as it's not a loop result
        if name == "der" and typ == "byte":
            continue
            
        # Check if the variable is actually returned in the function
        # by scanning ahead 
        is_returned = False
        in_loop = False
        for j in range(i+1, min(i+100, len(lines))):
            if "{" in lines[j] and "}" in lines[j]:
                pass # single line block
            if "for " in lines[j]:
                in_loop = True
            if re.search(r"return\b.*?\b" + name + r"\b", lines[j]) or re.search(r"return\b.*?\b[^,]*,\s*\b" + name + r"\b", lines[j]):
                is_returned = True
                break
            if re.search(r"^" + indent + r"}$", lines[j]):
                # reached end of block
                break
                
        # ERForeignKey has special logic (returned as part of struct)
        if name == "fks" and typ == "ERForeignKey":
            is_returned = True
            in_loop = True

        if is_returned and in_loop:
            lines[i] = f"{indent}{name} := []{typ}{{}}\n"

with open("internal/snowflake/client.go", "w") as f:
    f.writelines(lines)
