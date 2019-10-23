#!/usr/bin/python3

"""
This script does all necessary pre-processing to convert scdoc format into
Markdown.

Also, it generates Table of Contents for files with more than one
section.
If '; NO_TOC' line is present in file, ToC will not be generated.

And finally, '; TITLE Whatever' is converted into YAML front matter with 'title'
value.

Usage:
    prepare_md.py < in > out
    prepare_md.py file1 file2 file3
        Converts into _generated_file1.md, etc.
"""

import sys
import re

anchor_escape = str.maketrans(r' #()./\+-_', '__________')

def toc_entry(line, previous_h1_anchor):
    level = line.count('#')
    indent = '  ' * (level-1)
    name = line[line.rfind('#')+1:].strip().replace('_', r'\_')
    anchor = name.translate(anchor_escape).lower()
    if level == 2:
        anchor = previous_h1_anchor + '.' + anchor
    return '{indent}- [{name}](#{anchor})'.format(**locals()), anchor

def prepare(r, w):
    new_lines = list()
    h1_count = 0
    no_toc = False
    table_of_contents = list()
    title = str()
    previous_h1_anchor = ''

    inside_literal = False

    for line in r:
        if line.startswith('#') and not inside_literal:
            entry, anchor = toc_entry(line, previous_h1_anchor)
            table_of_contents.append(entry)
            new_lines.append('<a id="{}">\n\n'.format(anchor))
            if not line.startswith('##'):
                h1_count += 1
                previous_h1_anchor = anchor
            line = '#' + line

        if not inside_literal:
            if line.startswith('; NO_TOC'):
                no_toc = True
            if line.startswith('; TITLE '):
                title = line[8:].strip()
            if line[0] == ';':
                continue
            # turn *page*(1) into [**page(1)**](_generated_page.1.md)
            line = re.sub(r'\*(.+?)\*\(([0-9])\)', r'[*\1(\2)*](_generated_\1.\2.md)', line)
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
        print('---', file=w)
        print('title:', title, file=w)
        print('---', file=w)

    if h1_count > 1 and not no_toc:
        print('## Table of contents\n', file=w)
        print('\n'.join(table_of_contents), file=w)

    print(''.join(new_lines[1:]), file=w)

if len(sys.argv) == 1:
    prepare(sys.stdin, sys.stdout)
else:
    for f in sys.argv[1:]:
        new_name = '_generated_' + f[:-4] + '.md'
        prepare(open(f, 'r'), open(new_name, 'w'))
