# YF Intraday Option Chain Extractor

## Features

- Concurrent requests
- Read symbols from txt file
- Write to Postgres
- Docker
- Dedupe with index constraint
- Configurable poll interval (TODO)
- Error handling/retries (TODO)
- Proxy support (TODO)
- Intraday pull underlying (TODO)
- Scheduled with CRON


## TODO:
- scraping tests
- insert tests
- credentials

## Running:

Run postgres:
```
docker-compose up -d
```
Run app:

```
go run optionspy.go
```

Schedule with crontab like: `*/1 14-21 * * 1-5`
