# smartcollector

Smartcollector is a command line tool to export information about your SmartThings
sensors into [Prometheus](http://prometheus.io). The tool uses the [GoSmart](http://github.com/marcopaganini/gosmart) library to talk to SmartThings and collect sensor data, and emits
files ready to be imported by Prometheus' Node Exporter.

## Installation

The installation instructions assume a properly installed and configured Go
development environment. The very first step is to download and build
smartcollector (this step will also download and compile the GoSmart library):


```
$ go get -u github.com/marcopaganini/smartcollector
```

### GoSmart configuration

Before we can use smartcollector, we need to configure GoSmart. Follow the
[GoSmart installation instructions](https://github.com/marcopaganini/gosmart#installation)
carefully, making sure all steps have been followed.

With GoSmart configured,
[Follow the instructions](https://github.com/marcopaganini/gosmart#running-an-example) to
run the simple example that comes with GoSmart. Make sure the example displays
a list of your sensors on the screen.

### Smartcollector configuration

We now need to authorize smartcollector to access your Smartthings app. Take note of the `client_id` and `client_secret` of your SmartThings app (used when running the simple example above). Run:

```
$ smartcollector --client <client_id> --secret <client_secret> --textfile-dir "/tmp"
```

Follow the instructions to authorize the app (just like in the simple example.)

Smartcollector will write a file with your credentials to the home directory of the user
running it. After the first run, only the `client_id` is required to run smartcollector:

```
$ smartcollector --client <client_id> --textfile-dir "/tmp"
```

Please note the `textfile-dir` flag above. This instructs smartcollector to write
the "textfile" output into your "/tmp" directory. This makes it easy to test smartcollector
and perform the initial installation, but node-exporter (Prometheus) won't read and
import these files from "/tmp". In a production environment, you'll want to:

1. Make sure the `-collector.textfile.directory` flag of your `node_exporter` agrees with the
value set in `--textfile-dir` on the smartcollector. By default, smartcollector uses
"/run/textfile_collector"

1. Make sure the system user you use to run smartcollector has permissions to **write** under "textfile-dir".

1. Add an entry to your cron job to fetch the values every 5 or 10 minutes.

1. When everything is running well, you should start seeing a timeseries called `smartthings_sensors` in your prometheus console (usually, at [localhost:9090](http://localhost:9090)).
