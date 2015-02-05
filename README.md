## BOSH Agent [![Build Status](https://travis-ci.org/cloudfoundry/bosh-agent.png?branch=master)](https://travis-ci.org/cloudfoundry/bosh-agent)

* Documentation: [bosh.io/docs](https://bosh.io/docs)
* IRC: `#bosh` on freenode
* Google groups:
  [bosh-users](https://groups.google.com/a/cloudfoundry.org/group/bosh-users/topics) &
  [bosh-dev](https://groups.google.com/a/cloudfoundry.org/group/bosh-dev/topics) &
  [vcap-dev](https://groups.google.com/a/cloudfoundry.org/group/vcap-dev/topics) (for CF)

```
PATH=.../s3cli/out:$PATH bin/run -P ubuntu -C path_to_config.json
```

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
- S3
- DAV
- Dummy (for testing)

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
Run `bin/test-bosh-integration` to run those with your local agent changes.
However, in order to run the BOSH integrations tests, you will need a copy of the BOSH repo, which this script will do in `./tmp`.
BOSH uses Ruby for its tests, so you will also need to have that available.

You can run all the tests by running `bin/test`.

There is a stand-alone _BOSH Agent_ integration test to test config-drive using bosh-lite. This can be run locally via `integration/bin/test --provider=virtualbox`. The vagrant provider passed in can be changed to a provider of your choosing if you so desire.

#### Using IntelliJ with Go and the BOSH Agent

- Install [IntelliJ 13](http://www.jetbrains.com/idea/download/index.html) (we are using 13.0.1 Build 133.331)
- Install the latest [Google Go plugin for IntelliJ](https://github.com/go-lang-plugin-org/go-lang-idea-plugin). You may want to grab the latest early access (EAP) build, rather than the last release.
- (Optional) Download, Install & Select [improved keybindings](https://github.com/Pivotal-Boulder/IDE-Preferences) for IntelliJ:
    - `git clone git@github.com:Pivotal-Boulder/IDE-Preferences.git`
    - `cd ~/Library/Preferences/IntelliJIdea13/keymaps`
    - `ln -sf ~/workspace/IDE-Preferences/IntelliJKeymap.xml`
    - In IntelliJ: Preferences -> Keymap -> Pick 'Mac OS X 10.5+ Improved'
- Clone bosh-agent into a clean go workspace (or use a [bosh](https://github.com/cloudfoundry/bosh) clone with bosh/go as the workspace root):
    - `mkdir -p ~/workspace/bosh-agent-workspace/src/github.com/cloudfoundry`
    - `cd ~/workspace/bosh-agent-workspace/src/github.com/cloudfoundry`
    - `git clone https://github.com/cloudfoundry/bosh-agent`
- Open ~/workspace/bosh-agent-workspace as a new project in IntelliJ.
- Set the Go SDK as the Project SDK: 
    - Open the Project Structure window: `File -> Project Structure`
    - Select the `Project` tab in left sidebar
    - (Optional) Add a `New` Go SDK by selecting your go root. 
    - Select `Go SDK go1.3` under Project SDK
- Setup module sources
    - Open the Project Structure window: `File -> Project Structure`
    - Select the `Modules` tab in left sidebar
    - Select your module in the middle sidebar
    - Select the `Sources` tab in the Module pane
    - Select ~/workspace/bosh-agent-workspace/src and add is as a source dir
    - Select ~/workspace/bosh-agent-workspace/src/github.com/cloudfoundry/bosh-agent/Godeps and add is as an excluded dir
- Setup module dependencies
    - Open the Project Structure window: `File -> Project Structure`
    - Select the `Modules` tab in left sidebar
    - Select your module in the middle sidebar
    - Select the `Dependencies` tab in the Module pane
    - Select the `+ -> Jars or directories...` to add ~/workspace/bosh-agent-workspace/src/github.com/cloudfoundry/bosh-agent/Godeps/_workspace as a `sources` dependency
    - Rename the new dependency to `Godeps`
    - Use the arrow buttons to move `Godeps` above `Go SDK` and below `<Module source>`
- Set the bosh-agent dir as the Git root to enable version control
    - Select the `-` to remove the project root
    - Select the `+` to add the ~/workspace/bosh-agent-workspace/src/github.com/cloudfoundry/bosh-agent dir
- Install & configure the [Grep Console](https://github.com/krasa/GrepConsole) plugin
    - Install via `Preferences -> Plugins`
    - Select `Preferences -> Grep COnsole -> Enable ANSI coloring` to colorize Ginkgo test output
- Re-index your project: `File -> Invalidate Cache / Restart`

You should now be able to 'go to declaration', auto-complete, and run tests from within IntelliJ.

