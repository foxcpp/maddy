maddy(1) "maddy mail server" "maddy reference documentation"

; TITLE Command line arguments

# Name

maddy - Composable all-in-one mail server.

# Synopsis

*maddy* [options...]

# Description

Maddy is Mail Transfer agent (MTA), Mail Delivery Agent (MDA), Mail Submission
Agent (MSA), IMAP server and a set of other essential protocols/schemes
necessary to run secure email server implemented in one executable.

# Command line arguments

*-h, -help*
	Show help message and exit.

*-config* _path_
	Path to the configuration file. Default is /etc/maddy/maddy.conf.

*-libexec* _path_
	Path to the libexec directory. Helper executables will be searched here.
	Default is /usr/lib/maddy.

*-log* _targets..._
	Comma-separated list of logging targets. Valid values are the same as the
	'log' config directive. Affects logging before configuration parsing
	completes and after it, if the different value is not specified in the
	configuration.

*-debug*
	Enable debug log. You want to use it when reporting bugs.

*-v*
	Print version & build metadata.
