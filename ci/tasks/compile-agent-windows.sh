#!/bin/bash

set -e -x

VERSION=$(cat bosh-agent-zip-version/number)
COMPILED_AGENT_ZIP=$PWD/compiled-agent-zip
AGENT_DEPS_ZIP=$PWD/bosh-agent-deps-zip

export PATH=/usr/local/ruby/bin:/usr/local/go/bin:$PATH
export GOPATH=$(pwd)/gopath

apt-get update
apt-get -y install zip

cd gopath/src/github.com/cloudfoundry/bosh-agent

GOOS=windows ./bin/go build -o bosh-agent.exe main/agent.go
GOOS=windows ./bin/go build -o pipe.exe jobsupervisor/pipe/main.go

git rev-parse HEAD > ./commit

unzip -n $AGENT_DEPS_ZIP/agent-dependencies-v*.zip
mv ./job-service-wrapper.exe ./service_wrapper.exe

cat > ./service_wrapper.xml <<EOF
<service>
  <id>bosh-agent</id>
  <name>BOSH Agent</name>
  <description>BOSH Agent</description>
  <executable>bosh-agent.exe</executable>
  <arguments>-P windows -C agent.json -M windows</arguments>
  <logpath>/var/vcap/bosh/log</logpath>
  <log mode="roll-by-size">
  	<sizeThreshold>10240</sizeThreshold>
  	<keepFiles>8</keepFiles>
  </log>
  <onfailure action="restart" delay="5 sec"/>
</service>
EOF

cat > ./service_wrapper.exe.config <<EOF
<configuration>
  <startup>
    <supportedRuntime version="v4.0" />
  </startup>
</configuration>
EOF

RELEASE_ZIP=$PWD/bosh-windows-integration-v$VERSION.zip
zip ${RELEASE_ZIP} ./commit ./bosh-agent.exe ./pipe.exe ./service_wrapper.exe ./service_wrapper.xml ./service_wrapper.exe.config
mv ${RELEASE_ZIP} ${COMPILED_AGENT_ZIP}
