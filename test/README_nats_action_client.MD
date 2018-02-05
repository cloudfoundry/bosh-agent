Updated NATS client app - it now accepts 5 parameters through command line: <nats IP> <agent channel ID> <reply to ID> <command name> <command arguments string>. Agent ID can be retrieved from the agent settings file in /var/vcap/bosh
  example invocation ./nats_action_client 192.168.50.6 agent.XXX director.X.Y.Z mount_disk '["diskID","diskHint"]'

Usually nats_action_client is compiled for linux and ran from director's
/var/vcap/jobs/director/config (for nats certs/keys hardcoded into nats
client go app). Output on agent side can be seen by tailing
/var/vcap/bosh/log/current.
