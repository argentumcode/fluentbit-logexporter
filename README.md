# fluentbit-logexporter

A plugin for Fluent Bit that outputs the number of logs as Prometheus metrics.

## Configuration Parameters

The plugin supports the following configuration parameters:

| Key       | Description                                                                                             |
|-----------|---------------------------------------------------------------------------------------------------------|
| Labels    | Specifies the mapping between log entries and Prometheus labels.<br/>Format is label1=key1,label2=key2. |
| Listen    | Prometheus listen address and port. Default: `0.0.0.0:8681`                                             |
| View_Name | Prometheus metrics name. Default: `log_count`                                                           |
