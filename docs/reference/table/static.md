# Static table

The 'static' module implements table lookups using key-value pairs in its
configuration.

```
table.static {
	entry KEY1 VALUE1
	entry KEY2 VALUE2
	...
}
```

## Configuration directives

### entry _key_ _value_

Add an entry to the table.

If the same key is used multiple times, the last one takes effect.

