# Separate username and password lookup

auth.plain\_separate module implements authentication using username:password pairs but can
use zero or more "table modules" (maddy-tables(5)) and one or more
authentication providers to verify credentials.

```
auth.plain_separate {
	user ...
	user ...
	...
	pass ...
	pass ...
	...
}
```

How it works:
- Initial username input is normalized using PRECIS UsernameCaseMapped profile.
- Each table specified with the 'user' directive looked up using normalized
  username. If match is not found in any table, authentication fails.
- Each authentication provider specified with the 'pass' directive is tried.
  If authentication with all providers fails - an error is returned.

## Configuration directives

***Syntax:*** user _table module\_

Configuration block for any module from maddy-tables(5) can be used here.

Example:
```
user file /etc/maddy/allowed_users
```

***Syntax:*** pass _auth provider\_

Configuration block for any auth. provider module can be used here, even
'plain\_split' itself.

The used auth. provider must provide username:password pair-based
authentication.
