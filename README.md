# General idea
This is a command line interface for https://github.com/oscp/openshift-monitoring. It is used in our external monitoring system. It calls checks on the local server and reports them back to the monitoring tool. For more details about the check, visit https://github.com/oscp/openshift-monitoring.

## Usage & Configuration
Start from the configuration template 'config.template.yml'. Place your own config file at the same location as the binary then run:

```bash
openshift-monitoring-cli 
```

