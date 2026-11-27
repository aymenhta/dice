<div dir="rtl">

# نرد

نرد هو مخزن مفاتيح وقيم يعمل في الذاكرة، ولا يزال في مرحلة مبكرة من التطوير. نعمل حاليًا على بناء دعم لأوامر Redis الأكثر استخدامًا، مع هدف طويل المدى يتمثل في تسهيل الانتقال إليه بأقل قدر ممكن من التعديلات على التطبيقات الحالية.

لا يزال العمل جاريًا على توسيع نطاق التوافق، حيث نطمح إلى تحقيق توافق بنسبة 80% مع Redis الإصدار 8.10.3 بحلول إطلاق النسخة 1.0.


<div align="center">
<img 
    style="width: 400px; height: 400px; margin-bottom: 20px; border-radius:20px"
    src="./assets/logo.png" />
</div>
للتجربة قم بتشغيل الأداة عن طريق تنفيذ سكريبت البناء المرفق مع المستودع:


```bash
bash ./build_run.sh
```

بعدها قم بتنفيذ الأوامر التالية:


```shell
redis-cli -p 8888 SET action TEST
redis-cli -p 8888 GET action
redis-cli -p 8888 HSET user:1 name Aymen age 30
redis-cli -p 8888 HGETALL user:1
redis-cli -p 8888 LPUSH evens 2 4 6 8
redis-cli -p 8888 LRANGE evens 0 -1
redis-cli -p 8888 SADD fruits banana apple peach
redis-cli -p 8888 SMEMBERS fruits
```

كما يمكن استخدام أي عميل Redis متاح لأي لغة برمجة.

<div dir="ltr">

```python
import redis

r = redis.Redis("localhost", port=8888, db=0, decode_responses=True)

# Strings
r.set("user:100:name", "Alice", ex=3600) # expire in an hour
print(r.get("user:100:name")) # Output alice

users = {
    "user:101:name": "Alice",
    "user:102:name": "Bob",
    "user:103:name": "Charlie",
}
r.mset(users)

keys_to_fetch = ["user:101:name", "user:102:name", "user:999:name"]
print(r.mget(keys_to_fetch)) # Output: ['Alice', 'Bob', None]  (Missing keys return None)

# Hashes
r.hset("user:100:profile", mapping={"role": "admin", "active": "true"})
print(r.hgetall("user:100:profile"))

# Lists
r.rpush("tasks", "email", "report")
print(r.rpop("tasks"))

r.close()
```

</div>


## قائمة الأوامر المدعومة

- **الأوامر العامة**:
    - HELLO, PING, ECHO, EXISTS, DEL, EXPIRE, TTL, TYPE, DBSIZE, FLUSHDB, FLUSHALL, SELECT, QUIT. 
- **السلاسل النصية**: 
    - SET, GET, INCR, DECR, INCRBY, DECRBY, MGET, MSET, GETDEL, DELEX, GETEX, STRLEN, APPEND, DIGEST, GETRANGE, SETRANGE, MSETEX, MSETNX, SETNX.
- **جداول التجزئة**:
    - HSET, HGET, HGETALL, HDEL, HEXISTS, HLEN, HMGET, HKEYS, HVALS, HINCRBY, HEXPIRE.
- **القوائم**:
    - LPUSH, RPUSH, LPUSHX, RPUSHX, LPOP, RPOP, LRANGE, LLEN, LINDEX , LREM, LSET, LMOVE, LTRIM.
- **المجموعات**:
    - SADD, SREM, SISMEMBER, SMEMBERS, SCARD, SINTER, SUNION, SDIFF, SINTERSTORE, SUNIONSTORE, SDIFFSTORE, SINTERCARD, SRANDMEMBER, SPOP, SMOVE, SMISMEMBER.

## القيود الحالية

- لا يوجد حفظ دائم للبيانات على القرص (persistence).
- لا يوجد دعم للنسخ المتماثل (replication) ولا Pub/Sub ولا المعاملات (MULTI/EXEC) ولا المصادقة ولا TLS.
- انتهاء صلاحية المفاتيح يتم عند الوصول إليها (lazy expiration) فقط، دون مهمة خلفية تنظّف المفاتيح المنتهية.

- نحن حاليا ندعم العمليات على أنواع البيانات التالية فقط: **السلاسل النصية، وجداول التجزئة، والقوائم, والموجموعات.** سندعم تدريجيا بقية أنواع البيانات في Redis مع كل اصدار جديد. 
</div>

