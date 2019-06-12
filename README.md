# rdns-migrate-tools

Used to migrate rancher dns datum from 0.4.x to 0.5.x

## Download

[Binary](https://github.com/Jason-ZW/rdns-migrate-tools/releases/download/v1.0/rdns-migrate-tools)

## Build

`make`

## Migrate
```
./rdns-migrate-tools --src_endpoints=http://x.x.x.x:2379 --src_api_endpoint=http://x.x.x.x:9333 --dst_api_endpoint=http://x.x.x.x:9333
```

## Usages
```
NAME:
   rdns-migrate-tools - migrate RDNS from 0.4.x to 0.5.x('2019-06-12T05:31:04Z')

USAGE:
   rdns-migrate-tools [global options] command [command options] [arguments...]

VERSION:
   v1.0

AUTHOR(S):
   Rancher Labs, Inc.

COMMANDS:
     help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --debug, -d               used to set debug mode. [$DEBUG]
   --src_api_endpoint value  used to set source api endpoint which needs to be migrated. (default: "http://127.0.0.1:9333") [$src_API_ENDPOINT]
   --src_endpoints value     used to set source etcd endpoints which needs to be migrated. (default: "http://127.0.0.1:2379") [$SRC_ENDPOINTS]
   --src_prefix value        used to set source etcd prefix which needs to be migrated. (default: "/rdns") [$SRC_PREFIX]
   --src_domain value        used to set source domain which needs to be migrated. (default: "lb.rancher.cloud") [$SRC_DOMAIN]
   --dst_api_endpoint value  used to set destination api endpoint. [$DST_API_ENDPOINT]
   --dst_domain value        used to set destination domain. (default: "lb.rancher.cloud") [$DST_DOMAIN]
   --help, -h                show help
   --version, -v             print the version
```