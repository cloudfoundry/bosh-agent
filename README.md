## BOSH Agent [![Build Status](https://travis-ci.org/cloudfoundry/bosh-agent.png?branch=master)](https://travis-ci.org/cloudfoundry/bosh-agent)

    PATH=.../s3cli/out:$PATH bin/run -I dummy -P ubuntu


### Running locally

To start server locally:

    gem install nats
    nats-server

To subscribe:

    nats-sub '>' -s nats://localhost:4222

To publish:

    nats-pub agent.123-456-789 '{"method":"apply","arguments":[{"packages":[{"name":"package-name", "version":"package-version"}]}]}' -s nats://localhost:4222


### Blobstores

The Go Agent ships with 4 default blobstores:

- Local filesystem
- Dummy (for testing)
- S3
- DAV

You can, however, use custom blobstores by implementing a simple interface. For example, if you want to use a blobstore named "custom" you need to create an executable named `bosh-blobstore-custom` somewhere in `PATH`. This executable must conform to the following command line interface:

- `-c` flag that specifies a config file path (this will be passed to every call to the executable)
- must parse the config file in JSON format
- must respond to `get <blobID> <filename>` by placing the file identified by the blobID into the filename specified
- must respond to `put <filename> <blobID>` by storing the file at filename into the blobstore at the specified blobID

A full call might look like:

    bosh-blobstore-custom -c /var/vcap/bosh/etc/blobstore-custom.json get 2340958ddfg /tmp/my-cool-file


### Set up a workstation for development

Note: This guide assumes a few things:

- You have gcc (or an equivalent)
- You can install packages (brew, apt-get, or equivalent)

Get Golang and its dependencies (Mac example, replace with your package manager of choice):

- `brew update`
- `brew install go`
- `brew install git` (Go needs git for the `go get` command)
- `brew install hg` (Go needs mercurial for the `go get` command)

Clone and set up the BOSH Agent repository:

- `go get -d github.com/cloudfoundry/bosh-agent`
    - Note that this will print an error message because it expects a single package; our repository consists of several packages.
      The error message is harmless—the repository will still be checked out.
- `cd $GOPATH/src/github.com/cloudfoundry/bosh-agent`

From here on out we assume you're working in `$GOPATH/src/github.com/cloudfoundry/bosh-agent`

Install tools used by the BOSH Agent test suite:

- `bin/go get code.google.com/p/go.tools/cmd/vet`
- `bin/go get github.com/golang/lint/golint`

#### Running tests

Each package in the agent has its own unit tests. You can run all unit tests with `bin/test-unit`.

Additionally, [BOSH](https://github.com/cloudfoundry/bosh) includes integration tests that use this agent.
Run `bin/test-integration` to run those.
However, in order to run the BOSH integrations tests, you will need a copy of the BOSH repo, which this script will do in `./tmp`.
BOSH uses Ruby for its tests, so you will also need to have that available.

You can run all the tests by running `bin/test`.

#### Using IntelliJ with Go and the BOSH Agent

- Install [IntelliJ 13](http://www.jetbrains.com/idea/download/index.html) (we are using 13.0.1 Build 133.331)
- Set up the latest Google Go plugin for IntelliJ by following [Ross Hale's blog post](http://pivotallabs.com/setting-google-go-plugin-intellij-idea-13-os-x-10-8-5/) (the plugin found in IntelliJ's repository is dated)
- Download and use the [improved keybindings](https://github.com/Pivotal-Boulder/IDE-Preferences) for IntelliJ (optional):
    - `git clone git@github.com:Pivotal-Boulder/IDE-Preferences.git`
    - `cd ~/Library/Preferences/IntelliJIdea13/keymaps`
    - `ln -sf ~/workspace/IDE-Preferences/IntelliJKeymap.xml`
    - In IntelliJ: Preferences -> Keymap -> Pick 'Mac OS X 10.5+ Improved'

Set up the Go Agent project in IntelliJ:

- Open the ~/workspace/bosh-agent project in IntelliJ.
- Set the Go SDK as the Project SDK: File -> Project Structure -> Project in left sidebar -> Set the Go SDK go1.2 SDK under Project SDK
- Set the Go SDK as the Modules SDK: Modules in left sidebar -> Dependencies tab -> Set the Go SDK for the Module SDK -> Apply, OK

You should now be able to run tests from within IntelliJ.

