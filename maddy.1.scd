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
	Path to configuration file. Default is /etc/maddy/maddy.conf.
	
*-state* _path_
	Path to the state directory. This directory will be used to store all persistent
	data and should be writable. Default is /var/lib/maddy.

*-libexec* _path_
	Path to the libexec directory. Helper executables will be searched here.
	Default is /usr/lib/maddy.

*-runtime* _path_
	Path to the runtime directory. Temporary data will be stored here.
	It is not recommended to use tmpfs for it since runtime directory is used only 
	for large objects that can't be stored in RAM directly.

*-log* _targets..._
	Logging targets. Format is the same as 'log' config directive. 

*-debug*
	Enable debug log. You want to use it when reporting bugs.

*-debug.pprof, -debug.blockprofrate, -debug.mutexproffract*
	These options are meant for use by developers and not interesting for end users.

# AUTHORS

Maintained by Max Mazurov <fox.cpp@disroot.org>, who are assisted by other
open-source contributors. For more information about maddy development, see
https://github.com/foxcpp/maddy.

# SEE ALSO

*maddy.conf*(5)
