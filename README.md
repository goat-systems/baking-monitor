# Baking Monitor

Baking Monitor is a simple SMS notification service for Tezos Bakers that will text you when you miss a block or endorsement. It is built with [go-tezos](https://github.com/goat-systems/go-tezos). 

## Installation

### Docker
```
docker pull goatsystems/baking-monitor:latest

docker run --rm -ti goatsystems/baking-monitor:latest baking-monitor [command] \
-e BAKING_MONITOR_BAKER=<TODO (e.g. tz1SUgyRB8T5jXgXAwS33pgRHAKrafyg87Yc)> \
-e BAKING_MONITOR_ACCOUNT_SID=<TODO> \
-e BAKING_MONITOR_AUTH_TOKEN=<TODO> \
-e BAKING_MONITOR_FROM=<TODO (e.g. +12605557777)> \
-e BAKING_MONITOR_TO=<TODO (e.g. +12605557778, +12605557779)>
```

## Configuration

| ENV                                  | Description                                          | Default                       | Required |
|--------------------------------------|------------------------------------------------------|:-----------------------------:|:--------:|
| BAKING_MONITOR_BAKER                 | Pkh/Address of Baker                                 | N/A                           | True     |
| BAKING_MONITOR_ACCOUNT_SID           | Twilio Account SID                                   | N/A                           | True     |
| BAKING_MONITOR_AUTH_TOKEN            | Twilio Auth Token                                    | N/A                           | True     |
| BAKING_MONITOR_FROM                  | Twilio From Number                                   | N/A                           | True     |
| BAKING_MONITOR_TO                    | Recipient Phone Numbers (, seperated)                | N/A                           | True    |

## License

This project is licensed under the MIT License - see the [LICENSE.md](LICENSE.md) file for details
