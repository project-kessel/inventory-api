# Serializable Isolation Level in ResourceRepository

This document outlines the use of the `Serializable` isolation configuration in managing database transactions within the `ResourceRepository`. `Serializable` isolation ensures data consistency and integrity during concurrent operations by using automatic conflict detection and retries between transactions, specifically on our resource CRUD operations.

## Development Best Practices
When using the `Serializable` isolation level, it is crucial to follow best practices to minimize contention and ensure optimal performance.

1. **Serialization Failures Are Expected**: 
   - The `Serializable` isolation level is designed to detect conflicts between transactions. As a result, serialization failures may occur when two transactions attempt to modify the same data concurrently. This is a normal behavior and should be expected. Excessive retries may indicate contention issues.
   - When a serialization failure occurs, the transaction will be retried automatically up to the configured maximum number of retries.
1. **Reads Should Live In the Transaction**: 
   - All reads should be performed within the same transaction as your write to ensure that the data being read is used for conflict detection.
1. **Use Indexes**: 
   - Ensure that the database tables involved in the transactions are properly indexed. This can help reduce contention by allowing the database to quickly locate the data being accessed without scanning the entire table.
1. **Avoid Explicit Locks**: 
   - Explicit locks should be avoided. i.e. `SELECT FOR UPDATE` and `SELECT FOR SHARE`. These are not needed due to the protections automatically provided by serializable transactions. This is not a complete incompatibility, but if locks are being considered on top of SSI it could be indicative of design “smell”.
1. **Keep Transactions Lean**: 
   - Keep our transactions as lean as possible. Introducing more tables/queries to our resource transactions will increase our surface area for serialization failures. Increasingly complex transactions will produce additional stress on the DB under high load due to the conflict tracking mechanism that SSI uses

## Configuration

`max-serialization-retries` is available via storage config. The value holds the maximum number of retries for a transaction before it is aborted. The default value is `10`, but it can be adjusted if necessary. This should be done with caution, as increasing the number of retries may lead to performance degradation in high contention scenarios. 

**Before increasing the value we should analyze the root cause of the contention and address that if possible.**
```yaml
storage:
  ...
  max-serialization-retries: 10
```

## Root Causing Contention Issues
There are a few possible reasons for contention issues in a database:
1. **High Concurrency**: Multiple transactions trying to access the same data simultaneously can lead to contention.
    * Is a service provider making an unreasonable number of requests for specific or related resources?
    * Are there multiple service providers making many requests for the same resources?
2. **Long-Running Transactions**: Transactions that take a long time have more opportunities to conflict with others.
    * Was there new code added that may be causing long-running transactions?
    * You can check AWS performance insights for long-running queries.
4. **Lack of Indexing**: Missing indexes can cause queries to scan more data than necessary, leading to contention.
    * Are there new queries in transactions that are not using indexes?