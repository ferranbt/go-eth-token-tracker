
# Go-eth-token-tracker

Go-eth-token-tracker is a tracker for Ethereum ERC20 token transfers that stores the events in a PostgreSQL database. Besides, it exposes and http API to query the transfers. The tracker uses the Ethereum JsonRPC interface to bulk sync all the logs in the chain and watch for new events once it reaches the head.

The tracker makes an intensive use of the JSONRPC endpoint. By default, the tracker retrieves the logs in batches of 1000 elements. You can tune this value with the 'batch-size' depending on the limits and capabilities of your Ethereum endpoint. A value of 10000 should be enough to download all the transfers in less than a day. If using Infura (even with a low batch size), it is recommended to get an API token, otherwise you will hit limit usage very soon.

Besides the data stored in PostgreSQL, the token tracker also generates a [boltdb](https://github.com/boltdb/bolt) file with all the raw logs that emit a Transfer event. For the Ethereum mainnet, it sums up to a couple of dozens of GB.

## Usage

Run Postgresql as a backend to store the transfers:

```
docker run --net=host -v $PWD/postgresql-data:/var/lib/postgresql/data postgres
```

This command runs PostgreSQL as a docker container storing the data under ./postgresql-data in the host machine.

Run the tracker:

```
go run main.go [--config ./config.json] [args]
```

The tracker can be parametrized either with a config file or with command arguments.

This is an example of the default config file:

```
{
    "tracker": {
        "endpoint": "https://mainnet.infura.io",
        "dbpath": "data.db",
        "progressbar": true,
        "batchsize": 1000
    },
    "storage": {
        "endpoint": "user=postgres dbname=postgres sslmode=disable"
    },
    "http": {
        "addr": "127.0.0.1:5000"
    }
}
```

Some of this values can also be set on the CLI:

- http-addr: Endpoint (bind addr and port) for the HTTP Api. 

- jsonrpc-endpoint: Endpoint (http, ipc or ws) for the Ethereum JsonRPC provider.

- boltdb-path: File path for the internal tracker db.

- db-endpoint: Endpoint for the PostgreSQL storage.

- batch-size: Max batch size for the tracker JSONRPC getLogs queries.

- progress-bar: Show the progress bar (defaults to true).

- config: Path for the config file.

Note that this values will overwrite any values from the config file.

## Api

By default, the Rest API is created at 127.0.0.1:5000. It exposes the next endpoints:

- /tokens: List all the ERC20 tokens.

- /tokens/{token}?from=[addr1,addr2]&to=[addr3,addr4]: List all the transfers for 'token'. Filter by specific sources and destinations.

- /to/{address}?tokens=[token1,token2]&from=[addr1,addr2]: List all the token transfers to 'address'. Filter by specific tokens and sources.

- /from/{address}?tokens=[token1,token2]&to=[addr1,addr2]: List all the token transfers from 'address'. Filter by specific tokens and destinations.

All the endpoints work with pagination and only return the most recent 100 elements. You can use the offset and limit query parameters to paginate through the results (i.e. ?offset=2&limit=1000).
