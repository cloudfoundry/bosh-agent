#!/bin/bash
set -ex

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
OUTPUT_PATH=$DIR/bosh-agent.exe
CONFIG_PATH=$DIR/agent.json
SETTINGS_PATH=$DIR/settings.json
SERVICE_CONFIG=$DIR/service_wrapper.xml
AGENT_ID=${AGENT_ID:?"Need to set AGENT_ID"}

rm -f $OUTPUT_PATH $CONFIG_PATH $SETTINGS_PATH $SERVICE_CONFIG

GOOS=windows \
  go build \
  -o \
  $OUTPUT_PATH \
  github.com/cloudfoundry/bosh-agent/main

cat > $CONFIG_PATH <<EOF
{
  "Platform": {
    "Linux": {
      "DevicePathResolutionType": "scsi"
    }
  },
  "Infrastructure": {
    "Settings": {
      "Sources": [
        {
          "Type": "File",
          "SettingsPath": "C:\\\\vagrant\\\\settings.json"
        }
      ],
      "UseRegistry" : true
    }
  }
}
EOF

cat > $SETTINGS_PATH <<EOF
{
  "agent_id": "$AGENT_ID",
  "disks": {
    "system": "/dev/xvda",
    "ephemeral": "/dev/sdb",
    "persistent": {},
    "raw_ephemeral": null
  },
  "env": {
    "bosh": {
      "password": ""
    }
  },
  "networks": {
    "diego1": {
      "type": "",
      "ip": "10.10.7.11",
      "netmask": "255.255.255.0",
      "gateway": "10.10.7.1",
      "resolved": false,
      "use_dhcp": true,
      "default": ["dns", "gateway"],
      "dns": ["10.10.0.2"],
      "mac": "",
      "preconfigured": false
    }
  },
  "ntp": ["0.pool.ntp.org", "1.pool.ntp.org"],
  "mbus": "nats://192.168.60.1:4222",
  "vm": {
    "name": "vm-1f1aaed4-b479-4cf5-b73e-a7cbf0abf4ae"
  },
  "trusted_certs": ""
}
EOF

cat > $SERVICE_CONFIG <<EOF
<service>
  <id>bosh-agent</id>
  <name>BOSH Agent</name>
	<description>BOSH Agent</description>
  <executable>bosh-agent.exe</executable>
  <arguments>-P windows -C C:\\vagrant\\agent.json</arguments>
	<log mode="reset"/>
</service>
EOF

if vagrant status | grep default | grep running
then
	vagrant provision
else
	vagrant up --provider=virtualbox
fi
