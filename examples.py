import redis

r = redis.Redis("localhost", port=8888, db=0, decode_responses=True)

# Strings
r.set("user:100:name", "Alice", ex=3600) # expire in an hour
print(r.get("user:100:name")) # Output alice

# 1. Set multiple keys at once using a dictionary
users = {
    "user:101:name": "Alice",
    "user:102:name": "Bob",
    "user:103:name": "Charlie",
}
r.mset(users)

# 2. Get multiple keys at once (returns a list in the exact order requested)
keys_to_fetch = ["user:101:name", "user:102:name", "user:999:name"]
values = r.mget(keys_to_fetch)

print(values)
# Output: ['Alice', 'Bob', None]  (Missing keys return None)

# Hashes
r.hset("user:100:profile", mapping={"role": "admin", "active": "true"})
print(r.hgetall("user:100:profile"))

# Lists
r.rpush("tasks", "email", "report")
print(r.rpop("tasks"))

r.close()
