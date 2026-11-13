<div dir="rtl">

# نرد

قاعدة بيانات (مفتاح-قيمة) تعمل على الذاكرة مصممة لتكون ذات أداء عالي مع استهلاك قليل للموارد.

الهدف من المشروع هو بناء بديل خفيف وسريع لمجموعة أوامر Redis الأكثر استخداماً، مع كود واضح يسهل فهمه وتوسيعه.

للتجربة قم بتشغيل الأداة عن طريق تنفيذ سكريبت البناء المرفق مع المستودع:

```bash
bash build.sh
```

بعدها قم بتنفيذ الأوامر التالية:

```shell
redis-cli -p 8888 SET action TEST
redis-cli -p 8888 GET action
redis-cli -p 8888 HSET user:1 name Aymen age 30
redis-cli -p 8888 HGETALL user:1
redis-cli -p 8888 LPUSH evens 2 4 6 8
redis-cli -p 8888 LRANGE evens 0 -1
```


## قائمة الأوامر المدعومة

- **الأوامر العامة**:
    - HELLO, PING, ECHO, EXISTS, DEL, EXPIRE, TTL, TYPE, DBSIZE, FLUSHDB, FLUSHALL, SELECT, QUIT. 
- **السلاسل النصية**: 
    - SET, GET, INCR, DECR, INCRBY, DECRBY, MGET, MSET, GETDEL, DELEX, GETEX, STRLEN, APPEND, DIGEST, GETRANGE, SETRANGE.
- **التجزئات**:
    - HSET, HGET, HGETALL, HDEL, HEXISTS, HLEN, HMGET, HKEYS, HVALS, HINCRBY, HEXPIRE.
- **القوائم**:
    - LPUSH, RPUSH, LPUSHX, RPUSHX, LPOP, RPOP, LRANGE, LLEN, LINDEX , LREM, LSET, LMOVE, LTRIM.


## القيود الحالية

- لا يوجد حفظ دائم للبيانات على القرص (persistence).
- لا يوجد دعم للنسخ المتماثل (replication) ولا Pub/Sub ولا المعاملات (MULTI/EXEC) ولا المصادقة ولا TLS.
- انتهاء صلاحية المفاتيح يتم عند الوصول إليها (lazy expiration) فقط، دون مهمة خلفية تنظّف المفاتيح المنتهية.

- نحن حاليا ندعم العمليات على أنواع البيانات التالية فقط: **السلاسل النصية، والتجزئات، والقوائم.** سندعم تدريجيا بقية أنواع البيانات في Redis مع كل اصدار جديد. 
</div>
