# OpenMetrics/Prometheus telemetry

Various server statistics are provided in OpenMetrics format by the
"openmetrics" module.

To enable it, add the following line to the server config:

```
openmetrics tcp://127.0.0.1:9749 { }
```

Scrape endpoint would be `http://127.0.0.1:9749/metrics`.

## Metrics

```
# AUTH command failures due to invalid credentials.
maddy_smtp_failed_logins{module}
# Failed SMTP transaction commands (MAIL, RCPT, DATA).
maddy_smtp_failed_commands{module, command, smtp_code, smtp_enchcode}
# Messages rejected with 4xx code due to ratelimiting.
maddy_smtp_ratelimit_deferred{module}
# Amount of started SMTP transactions started.
maddy_smtp_started_transactions{module}
# Amount of aborted SMTP transactions started.
maddy_smtp_aborted_transactions{module}
# Amount of completed SMTP transactions.
maddy_smtp_completed_transactions{module}
# Number of times a check returned 'reject' result (may be more than processed
# messages if check does so on per-recipient basis).
maddy_check_reject{check}
# Number of times a check returned 'quarantine' result (may be more than
# processed messages if check does so on per-recipient basis).
maddy_check_quarantined{check}
# Amount of queued messages.
maddy_queue_length{module, location}
# Outbound connections established with specific TLS security level.
maddy_remote_conns_tls_level{module, level}
# Outbound connections established with specific MX security level.
maddy_remote_conns_mx_level{module, level}
```
