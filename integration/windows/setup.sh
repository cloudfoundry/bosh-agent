#!/bin/bash
set -x

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
OUTPUT_PATH=$DIR/bosh-agent.exe
CONFIG_PATH=$DIR/agent.json

rm $OUTPUT_PATH
rm $CONFIG_PATH

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

vagrant provision
