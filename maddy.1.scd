maddy(1) "maddy mail server"

# NAME

maddy - simple SMTP+IMAP mail server

# SYNOPSIS

*maddy* [options...]

# DESCRIPTION

Maddy implements Mail Transfer agent (MTA), Mail Submission Agent (MSA) and
IMAP server functionality in one application.

# OPTIONS

*-h, -help*
	Show help message and exit.

*-config* _path_
	Path to configuratio file. Default is /etc/maddy/maddy.conf

# ENVIRONMENT VARIABLES

*MADDYSTATE*
	Can be used to override state directory. Does have higher priority than
	statedir in configuration.

*MADDYLIBEXEC*
	Can be used to override libexec directory. Does have higher prirotiy than
	libexecdir in configuration.

# AUTHORS

Maintained by Simon Ser <contact@emersion.fr> and Max Mazurov
<fox.cpp@disroot.org>, who are assisted by other open-source contributors. For
more information about maddy development, see
https://github.com/emersion/maddy.

# SEE ALSO

*maddy.conf*(5)
