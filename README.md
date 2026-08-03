# SQUASH IT: URL Shortening service in GO
### What is Squash it?
Squash it is a url shortening service that is built with extensibility in mind.
Its name is a metaphorical representation for what happens with large urls in shortening services.

They get squashed!

# Table of contents
- [Architecture behind Squash-it](#architecture-behind-squash-it)
- [Architecture Diagram](#high-level-architecture-diagram)
- [Running The App]()
- [External Packages Used](#external-packages-used)
- [Custom Components](#custom-components)
- [Hash Choice](#hash-choice)
- [User Token Bucket](#user-token-bucket)
- [Nuances](#nuances)
- [Scaling considerations](#scaling-consideration)
  - [Observability](#observability)
- [On Bloom filter](#on-bloom-filter)
- [AI Usage](#ai-usage)

## Architecture behind Squash-it

- An HTTP router built on top of `net/http` (inspired largely by cloudwego/hertz amazing http server).
- A bloom filter for quick inspection if the service carries a hash before it moves to data structures in or out of the network. ([Read more](#on-bloom-filter)).
- An LRU (Least Recently Used) Cache as L1 cache for quick cache hits (this could be the main and only cache if you decide to drop the Redis L2).
- An L2 Redis Cache. The app absolutely does NOT need redis to run. It safely and silently degrades to only LRU in case of no redis connectivity.
- A simple SQLite database with a single table. This is the main choice over NoSQL because of the simplicity it offers for this demo / assignment.

## High Level Architecture Diagram

![architecture-diagram](https://i.ibb.co/v8HD8rt/nse-236004746501201759-Notes-260803-202100-jpg.jpg)

### What is happening?

### **`/encode` Flow (Long URL -> Short URL):**
- The user's request first hits the rate limiter
- The DTO is parsed and sent to the service
- The service generates an 8 character murmur hash inside a retry loop for collision resolution
- The hash is evaluated against Bloom filter. 
  - If it doesn't exist an idempotent executeInsert method is called on the hashToken and the longURL. Returning the final hashToken to the user
  - If bloom asserts FP (Maybe exists). We issue a lookup into the database (by the hash / pathHash). Check if the returned longURL matches the user's intention:
    - If it does return the existing url
    - If it doesn't then a collision with different longURLs happened, we continue to retry with another hash.
- This loop continues until we either create or find a proper pathHash matching the longURL provided. Or we run out of retries and exit the service so we don't hash indefinitely.

### **`/decode` Flow (pathHash -> LongURL):**
- The provided hash is evaluated against the bloom filter.
  - Not Found path: Fail fast - this is protection against spamming or bot attempts. If the user rencodes the long URL using the encode flow and get the same pathHash, bloom will be updated and the next `/decode` call will converge into the below
  - Maybe Found: Check the cache pipeline:
    - Found: Return it
    - Not Found: Query the database.
      - Found: Update the cache pipeline, bloom and return the longURL back to the user.
      - Not Found: return error

### **The additional Visit URL path:**

This route takes in the full url. Does the same as the decode flow except:
- update the click_count metric in the database
- Redirect the user to the longURL using a `302 Found` code so that the browser doesn't cache the Location header.

## Running The App
The app can be run in 2 modes:
- **Isolated without any over-network dependencies (Like Redis) - Simply clone the repo and run:**

```aiignore
go run main.go
```
- With L2 Cache (REDIS). This will spin up a redis container + build the app in a 2-step distroless image:

```aiignore
docker compose up -d
```

## Testing

Test coverage was included for the majority of required cases. Some edge cases are not explicitly tested such as:
- What happens when a 2 goroutines get the same hash for the same. One gorotuine will succeed and the other will face a slient CONFLICT on write due to (sql: long_url unique constraint). The system will gracefully pass the hashToken created from the first goroutine.

To run tests:
```aiignore
go test -v -race ./...
```

## External Packages Used

- Bloom Filter package [bits-and-blooms/bloom](https://github.com/bits-and-blooms/bloom)
- Redis client for go [redis/go-redis](https://github.com/redis/go-redis)
- Murmur Hash Package (More on that below) [twmb/murmur3](https://github.com/twmb/murmur3)
- Go's Rate package for rate limiting [time/rate](https://pkg.go.dev/golang.org/x/time/rate)
- SQLite Driver for go. Modernc is used here to enable CGO_ENABLE=0 in Docker build [modernc/sqlite](https://pkg.go.dev/modernc.org/sqlite)

## Custom Components

- Custom router built on top of `net/http` in lieu of using a third party library like Hertz.
- LRU Cache.
- User Token Bucket custom per-user orchestration wrapping `x/time/rate.Limiter` per client IP for /encode, /decode rate limiting ([Read more](#user-token-bucket)).
- The entire bloom filter initialization and serialization steps.
- The URL Repository.
- Services, Handlers, Middlewares.

Basically, all abstractions were intentional to be able to create an extensible application beyond just one instance.

## Hash Choice
When presented with the problem statement in the assignment. The initial thought wasn't which hashing algorithm produces the least collisions.
It was "when presented with a collision - what's the least work I can do to re-hash the value". 

At a high level: 
- FNV-1a creates messy engineering overhead because a collision forces you to manually alter your database logic or append text salts to the URL.
- Murmur handles collisions simply by changing a seed number. This built-in seeding keeps your production code clean, fast, and highly scalable without complex structural workarounds.

Both are, for this use-case, better in CPU utilization under high-throughput systems than cryptographic algorithms such as SHA-256.

## User Token Bucket

To make a service such as a url Shortener a little resistant against bots hammering our endpoints to `/encode`, there needs to be a throttling mechanism.
This service uses a token bucket algorithm which refills a bucket based on the (refill_rate x &Delta; time).

`type UserTokenBucket struct`

stores a:
- map of user identifier pointing to a 
`*tokenBucket` reference.
- tokens `burstLimit` which is how many tokens the user is allowed to exhaust per instant before we throttle.

The Bucket also performs a lazy cleanup loop triggered by the `Allow(user string) bool` instead of spinning up a ticker so that if we're sitting idle with no hits, our CPU cycles are not exhausted. Only 1 goroutine can perform a cleanup at a time by using atomic flags making it efficient

## Nuances
The real nuances I hovered over, but explicitly left to gracefully be allowed in the system:

- Same urls, different structures: Assume `https://www.domain.com/utm_source=5&utm_campaign=4` and `https://www.domain.com/utm_campaign=4&utm_source=5`. Same url - at the current state it will get 2 different hashes. But an enhancement that was deliberately deferred (because this digs into implications of user intent) is to breakdown URL components by `url.Parse` and order the keys and construct an ordered string. If both match, we have a duplicate url shortening attempt, and we can gracefully deduplicate (return the existing short url). 
- TTL deliberately lives in the `RedisCache` constructor rather than the shared Cache interface since this LRU implementation doesn't honor TTL, an interface promising it would be a broken contract for that implementation to satisfy.

## Scaling consideration

When considering scaling of this specific service:

- Redis no longer becomes an option. It's mandatory.
Scaling also implicitly implies need for it (higher traffic and more throughput requirements).

- More throughput equals risk of pool starvation from the database layer such as postgres. Bloom doesn't become a luxury, it becomes an essential guard.
A distributed bloom filter on redis (via RedisBloom) would be the choice, with possibility for local sync with in-app bloom instance.

- Token buckets also need their distributed representation per user.

- LRU (or maybe an LFU) remains a valuable in-memory structure to reduce cache misses for as much as possible.

- Viral or "promoted" (if monetized) urls also might need a special handling in terms of rate limiting - not per user - but per url + user combination.

#### Observability
Throughout the service, it was made sure that `context.Context` is glorified for the exact purpose of this section. Scaling. 

Which is why the abstraction over `net/http` was created for this project. To provide `context.Context` cleanly as the first parameter in route handler function `router.HandlerFunc`. 

If we're scaling the service, propagating context throughout the application doesn't only make it listen for cancellation signals, but also allow observability frameworks like **OpenTelemetry** and distributed tracing systems like **Jaeger** to trace spans across application components and their offshore distributed sidecars or peers.

## On Bloom Filter
> I Keep asking. What if Bloom Doesn't Bloom Enough?

A bloom filter is really as robust as its startup. A bloom's filter guarantee is providing definitive assertion if a key doesn't exist. If, for any reason the filter goes out of sync, it will begin providing false negatives.

To combat this, the service treats negative assertions in 2 ways:
1. Creating a new record is idempotent. Meaning if bloom returns a FN (False Negative) and we proceed to create. Our `FindOrCreate` method simply propagates existing record back to bloom.
2. Reading a hash that doesn't exist in the bloom filter will trigger a `404 Not Found`. This has been intentionally left as is, because of point #1. If a user doesn't find their url, they will possibly attempt to reshorten it resulting in the same hash that already exists in database. This however reduces service quality.

Considerations for #2 were thought of, but not implemented due to scope. One possible remedy is to dynamically take down bloom when it begins returning false negatives, reconstruct it asynchronously, and repoint to the new version.

Additionally, Squash-it does the following:
- Check if there is a binary file that can be used to construct the filter on app startup.
- If there isn't, instantiate a brand-new instance (this assumes first app heartbeat).
- If there is: begin loading the binary into a new filter using a goroutine, meanwhile any hits to bloom return a False Positive, triggering application cache/db lookups. For Add() method, we maintain a backlog of keys added since construction. Once the goroutine finishes, it takes the backlog, adds it sequentially into the filter and locks to swap the pointers in Bloom.bf with the reconstructed up-to-date version.
- On the side we create a backup of the binary every N interval by a background goroutine.
- If application recieves a SIGTERM, we wait for 30 seconds or until we take the latest copy of the binary and save it to disk.

This is best-effort. One thing that could be done to enhance it:

- If we recieve a shutdown while background reconstruction is happening. Instead of serializing and overwriting our previous bloom binary with an empty bloom filter. We serialize the backlog and pick it up when we restart.

## AI Usage
AI assistance was limited to targeted lookups. SQLite syntax I don't use daily, and confirming the correct `net/http` approach for extracting client IP. All architecture, design decisions, and implementation are my own, including anything wrong with them.
