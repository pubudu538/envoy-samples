# Envoy External Processing Filter

A really basic implementation of envoy [External Processing Filter](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/filters/http/ext_proc/v3alpha/ext_proc.proto#external-processing-filter).This capability allows you to define an external gRPC server which can selectively process headers and payload/body of requests (see [External Processing Filter PRD](https://docs.google.com/document/d/1IZqm5IUnG9gc2VqwGaN5C2TZAD9_QbsY9Vvy5vr9Zmw/edit#heading=h.3zlthggr9vvv).  Basically, your own unrestricted filter.

![Alt text](ext_process_filter.jpg?raw=true "External Processing Filter")

---
### Start the Containers

```
docker compose up
```

### Scenario 1

1. Request Path: 
      - Header Path: Drop Authorization header
      - Body Path: No Change
2. Response Path: 
      - Header Path: Drop Access control headers and Set x-server header
      - Body Path: No Change

**Request:**
```
curl -X GET http://localhost -H "Authorization: Bearer testtoken"  -v
```

**Response:**
```
> GET / HTTP/1.1
> Host: localhost
> User-Agent: curl/7.64.1
> Accept: */*
> Authorization: Bearer testtoken
>
< HTTP/1.1 200 OK
< server: envoy
< date: Wed, 18 Aug 2021 11:01:09 GMT
< content-type: application/json
< x-envoy-upstream-service-time: 2
< x-server: envoy-proxy
< transfer-encoding: chunked
<
{
  "args": {},
  "data": "",
  "files": {},
  "form": {},
  "headers": {
    "Accept": "*/*",
    "Host": "localhost",
    "User-Agent": "curl/7.64.1",
    "X-Envoy-Expected-Rq-Timeout-Ms": "15000",
    "X-Envoy-Original-Path": "/"
  },
  "json": null,
  "method": "GET",
  "origin": "172.19.0.4",
  "url": "http://localhost/anything"
}
```

### Scenario 2

1. Request Path:
   - Header Path: Drop Authorization header and append content-length header
   - Body Path: Append some content to the body
2. Response Path:
   - Header Path: Drop Access control headers and Set x-server header
   - Body Path: Append some content to the body

**Request:**
```
curl -X POST http://localhost -H "Authorization: Bearer testtoken" -d {"abc:xyz"} -v
```

**Response:**
```
> POST / HTTP/1.1
> Host: localhost
> User-Agent: curl/7.64.1
> Accept: */*
> Authorization: Bearer testtoken
> Content-Length: 9
> Content-Type: application/x-www-form-urlencoded
>
* upload completely sent off: 9 out of 9 bytes
< HTTP/1.1 200 OK
< server: envoy
< date: Wed, 18 Aug 2021 11:23:29 GMT
< content-type: application/json
< x-envoy-upstream-service-time: 7
< x-server: envoy-proxy
< transfer-encoding: chunked
<
{
  "args": {},
  "data": "",
  "files": {},
  "form": {
    "{abc:xyz} baaar ": ""
  },
  "headers": {
    "Accept": "*/*",
    "Content-Type": "application/x-www-form-urlencoded",
    "Host": "localhost",
    "Transfer-Encoding": "chunked",
    "User-Agent": "curl/7.64.1",
    "X-Envoy-Expected-Rq-Timeout-Ms": "15000",
    "X-Envoy-Original-Path": "/"
  },
  "json": null,
  "method": "POST",
  "origin": "172.19.0.4",
  "url": "http://localhost/anything"
}
* Connection #0 to host localhost left intact
 pubci*
```
--- 
## Dev Guide

1. Build the external process filter implementation service

    ```
    docker build -t pubudu/ext-process-filter-envoy:v1 .
    ```
   
2. Start the containers

    ```
    docker compose up
    ```

### Source https://github.com/salrashid123/envoy_ext_proc
