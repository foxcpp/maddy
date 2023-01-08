#!/usr/bin/python3

"""
This script does all necessary pre-processing to convert scdoc format into
Markdown.

Usage:
    prepare_md.py < in > out
    prepare_md.py file1 file2 file3
        Converts into _generated_file1.md, etc.
"""

import sys
import re

anchor_escape = str.maketrans(r' #()./\+-_', '__________')

def prepare(r, w):
    new_lines = list()
    title = str()
    previous_h1_anchor = ''

    inside_literal = False

    for line in r:
        if not inside_literal:
            if line.startswith('; TITLE ') and title == '':
                title = line[8:]
            if line[0] == ';':
                continue
            # turn *page*(1) into [**page(1)**](../_generated_page.1)
            line = re.sub(r'\*(.+?)\*\(([0-9])\)', r'[*\1(\2)*](../_generated_\1.\2)', line)
            # *aaa* => **aaa**
            line = re.sub(r'\*(.+?)\*', r'**\1**', line)
            # remove ++ from line endings
            line = re.sub(r'\+\+$', '<br>', line)
            # turn whatever looks like a link into one
            line = re.sub(r'(https://[^ \)\(\\]+[a-z0-9_\-])', r'[\1](\1)', line)
            # escape underscores inside words
            line = re.sub(r'([^ ])_([^ ])', r'\1\\_\2', line)

        if line.startswith('```'):
            inside_literal = not inside_literal

        new_lines.append(line)

    if title != '':
        print('#', title, file=w)

    print(''.join(new_lines[1:]), file=w)

if len(sys.argv) == 1:
    prepare(sys.stdin, sys.stdout)
else:
    for f in sys.argv[1:]:
        new_name = '_generated_' + f[:-4] + '.md'
        prepare(open(f, 'r'), open(new_name, 'w'))
