ideas so far
- best is to use disk-uuid instead of disk-id but this is not currently provided by in the metadata no idea how we can get those
- strip the disk-id of all prefixes for example `google-disk-8df5ef90-4132-4314-7ee3-e4224adf27cb` will be `8df5ef90-4132-4314-7ee3-e4224adf27cb`
  and we can then set the prefixes in the stemcell. (least ammount of work and backwards compatible. but still looks like ducktape)

IDS are coming from `/var/vcap/bosh/persistent_disk_hints.json`

# IAAS

## GCP:
contents of `persistent_disk_hints.json`
```
{
  "disk-69e53b13-76b3-4400-4437-633782a5cbb5": {
    "ID": "disk-69e53b13-76b3-4400-4437-633782a5cbb5",
    "DeviceID": "",
    "VolumeID": "/dev/sdb",
    "Lun": "",
    "HostDeviceID": "",
    "Path": "/dev/sdb",
    "ISCSISettings": {
      "InitiatorName": "",
      "Username": "",
      "Target": "",
      "Password": ""
    },
    "FileSystemType": "",
    "MountOptions": null,
    "Partitioner": ""
  }
}
```

files listed in `/dev/disk/by-id`
```
google-disk-69e53b13-76b3-4400-4437-633782a5cbb5
google-disk-69e53b13-76b3-4400-4437-633782a5cbb5-part1
google-persistent-disk-0
google-persistent-disk-0-part1
google-persistent-disk-0-part2
google-persistent-disk-0-part3
google-persistent-disk-0-part4
scsi-0Google_PersistentDisk_disk-69e53b13-76b3-4400-4437-633782a5cbb5
scsi-0Google_PersistentDisk_disk-69e53b13-76b3-4400-4437-633782a5cbb5-part1
scsi-0Google_PersistentDisk_persistent-disk-0
scsi-0Google_PersistentDisk_persistent-disk-0-part1
scsi-0Google_PersistentDisk_persistent-disk-0-part2
scsi-0Google_PersistentDisk_persistent-disk-0-part3
scsi-0Google_PersistentDisk_persistent-disk-0-part4
```

## AWS
contents of `persistent_disk_hints.json`
```
{
  "vol-04b2a40747d7e9625": {
    "ID": "vol-04b2a40747d7e9625",
    "DeviceID": "",
    "VolumeID": "/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol04b2a40747d7e9625",
    "Lun": "",
    "HostDeviceID": "",
    "Path": "/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol04b2a40747d7e9625",
    "ISCSISettings": {
      "InitiatorName": "",
      "Username": "",
      "Target": "",
      "Password": ""
    },
    "FileSystemType": "",
    "MountOptions": null,
    "Partitioner": ""
  }
}
```

files listed in `/dev/disk/by-id`
```
nvme-Amazon_Elastic_Block_Store_vol04b2a40747d7e9625
nvme-Amazon_Elastic_Block_Store_vol04b2a40747d7e9625_1
nvme-Amazon_Elastic_Block_Store_vol04b2a40747d7e9625_1-part1
nvme-Amazon_Elastic_Block_Store_vol04b2a40747d7e9625-part1
nvme-Amazon_Elastic_Block_Store_vol0c2ee6da5d33b8f8b
nvme-Amazon_Elastic_Block_Store_vol0c2ee6da5d33b8f8b_1
nvme-Amazon_Elastic_Block_Store_vol0c2ee6da5d33b8f8b_1-part1
nvme-Amazon_Elastic_Block_Store_vol0c2ee6da5d33b8f8b_1-part2
nvme-Amazon_Elastic_Block_Store_vol0c2ee6da5d33b8f8b-part1
nvme-Amazon_Elastic_Block_Store_vol0c2ee6da5d33b8f8b-part2
nvme-Amazon_Elastic_Block_Store_vol0c67d3300b92fe851
nvme-Amazon_Elastic_Block_Store_vol0c67d3300b92fe851_1
nvme-Amazon_Elastic_Block_Store_vol0c67d3300b92fe851_1-part1
nvme-Amazon_Elastic_Block_Store_vol0c67d3300b92fe851-part1
nvme-nvme.1d0f-766f6c3034623261343037343764376539363235-416d617a6f6e20456c617374696320426c6f636b2053746f7265-00000001
nvme-nvme.1d0f-766f6c3034623261343037343764376539363235-416d617a6f6e20456c617374696320426c6f636b2053746f7265-00000001-part1
nvme-nvme.1d0f-766f6c3063326565366461356433336238663862-416d617a6f6e20456c617374696320426c6f636b2053746f7265-00000001
nvme-nvme.1d0f-766f6c3063326565366461356433336238663862-416d617a6f6e20456c617374696320426c6f636b2053746f7265-00000001-part1
nvme-nvme.1d0f-766f6c3063326565366461356433336238663862-416d617a6f6e20456c617374696320426c6f636b2053746f7265-00000001-part2
nvme-nvme.1d0f-766f6c3063363764333330306239326665383531-416d617a6f6e20456c617374696320426c6f636b2053746f7265-00000001
nvme-nvme.1d0f-766f6c3063363764333330306239326665383531-416d617a6f6e20456c617374696320426c6f636b2053746f7265-00000001-part1
```

## OpenStack:
contents of `persistent_disk_hints.json`
```
{
  "05c185c6-e8a3-4216-ab30-1f5cf7d17f0d": {
    "ID": "05c185c6-e8a3-4216-ab30-1f5cf7d17f0d",
    "DeviceID": "",
    "VolumeID": "/dev/sdb",
    "Lun": "",
    "HostDeviceID": "",
    "Path": "/dev/sdb",
    "ISCSISettings": {
      "InitiatorName": "",
      "Username": "",
      "Target": "",
      "Password": ""
    },
    "FileSystemType": "",
    "MountOptions": null,
    "Partitioner": ""
  }
}
```

files listed in `/dev/disk/by-id`
```
scsi-305c185c6e8a34216ab301f5cf7d17f0d
scsi-305c185c6e8a34216ab301f5cf7d17f0d-part1
scsi-36000c295e04c2c9a4b447d59632b887b
scsi-36000c295e04c2c9a4b447d59632b887b-part1
scsi-36000c295e04c2c9a4b447d59632b887b-part2
scsi-36000c295e04c2c9a4b447d59632b887b-part3
wwn-0x05c185c6e8a34216ab301f5cf7d17f0d
wwn-0x05c185c6e8a34216ab301f5cf7d17f0d-part1
wwn-0x6000c295e04c2c9a4b447d59632b887b
wwn-0x6000c295e04c2c9a4b447d59632b887b-part1
wwn-0x6000c295e04c2c9a4b447d59632b887b-part2
wwn-0x6000c295e04c2c9a4b447d59632b887b-part3
```

## Azure:
contents of `persistent_disk_hints.json`
```
{
  "caching:None;disk_name:bosh-disk-data-65216cfc-56cf-4c6d-86d8-92db25f7c555;resource_group_name:rg-bosh-lod-29": {
    "ID": "caching:None;disk_name:bosh-disk-data-65216cfc-56cf-4c6d-86d8-92db25f7c555;resource_group_name:rg-bosh-lod-29",
    "DeviceID": "",
    "VolumeID": "",
    "Lun": "1",
    "HostDeviceID": "{f8b3781b-1e82-4818-a1c3-63d806ec15bb}",
    "Path": "",
    "ISCSISettings": {
      "InitiatorName": "",
      "Username": "",
      "Target": "",
      "Password": ""
    },
    "FileSystemType": "",
    "MountOptions": null,
    "Partitioner": ""
  }
}
```
files listed in `/dev/disk/by-id`
```
scsi-3600224802f9637138f32cc2d44e8c34e
scsi-3600224802f9637138f32cc2d44e8c34e-part1
scsi-3600224802f9637138f32cc2d44e8c34e-part2
scsi-3600224803c4aeae9430e9c897b3979d7
scsi-3600224803c4aeae9430e9c897b3979d7-part1
scsi-3600224804c5eb34c6e0c024d5289763d
scsi-3600224804c5eb34c6e0c024d5289763d-part1
wwn-0x600224802f9637138f32cc2d44e8c34e
wwn-0x600224802f9637138f32cc2d44e8c34e-part1
wwn-0x600224802f9637138f32cc2d44e8c34e-part2
wwn-0x600224803c4aeae9430e9c897b3979d7
wwn-0x600224803c4aeae9430e9c897b3979d7-part1
wwn-0x600224804c5eb34c6e0c024d5289763d
wwn-0x600224804c5eb34c6e0c024d5289763d-part1
```

## AliCloud:
contents of `persistent_disk_hints.json`
```
{
  "d-gw8ibkvqdrmdqzwpo4rk": {
    "ID": "d-gw8ibkvqdrmdqzwpo4rk",
    "DeviceID": "",
    "VolumeID": "/dev/disk/by-id/virtio-gw8ibkvqdrmdqzwpo4rk",
    "Lun": "",
    "HostDeviceID": "",
    "Path": "/dev/disk/by-id/virtio-gw8ibkvqdrmdqzwpo4rk",
    "ISCSISettings": {
      "InitiatorName": "",
      "Username": "",
      "Target": "",
      "Password": ""
    },
    "FileSystemType": "",
    "MountOptions": null,
    "Partitioner": ""
  }
}
```
files listed in `/dev/disk/by-id`
```
virtio-gw8a1q0dqyqmcjrnetpj
virtio-gw8a1q0dqyqmcjrnetpj-part1
virtio-gw8a1q0dqyqmcjrnetpj-part2
virtio-gw8au7up14j66ge6u4vs
virtio-gw8au7up14j66ge6u4vs-part1
virtio-gw8ibkvqdrmdqzwpo4rk
virtio-gw8ibkvqdrmdqzwpo4rk-part1
```