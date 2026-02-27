## Core Concepts

### Synchronous Communication

Most systems use synchronous communication (HTTP/REST, gRPC):

- Client sends a request
- Server processes it and returns a response
- Client **waits** for the response before continuing

**Problem:** When multiple synchronous calls are chained in a single action, any failure in the chain blocks or breaks the whole operation.

### Asynchronous Communication

Event-driven patterns decouple operations so that non-critical work does not block the critical path.

- Work is dispatched and executed independently
- The caller does not wait for completion
- Failures are handled by retrying in the background

---

## Problem Illustration

### Scenario: User Sign-Up

A user signs up. The system must:

1. Save their account to the database (**critical**)
2. Add them to the newsletter (**non-critical, eventual**)
3. Send a welcome notification (**non-critical, eventual**)

### Synchronous implementation (problematic)

```go
func SignUp(u User) error {
    if err := CreateUserAccount(u); err != nil {
        return err
    }

    if err := AddToNewsletter(u); err != nil {
        return err
    }

    if err := SendNotification(u); err != nil {
        return err
    }

    return nil
}
```

**Failure modes:**

- If `AddToNewsletter` or `SendNotification` fails, you must choose between:
  - Returning an error and blocking the user from signing up (bad UX)
  - Ignoring errors and leaving data in an inconsistent state (bad data integrity)

**Root cause:** Non-critical operations are blocking the critical path.

---

## Solution: Naive Async with Goroutines

### Basic goroutine dispatch

```go
go func() {
    if err := AddToNewsletter(u); err != nil {
        log.Printf("failed to add user to newsletter: %v", err)
    }
}()
```

The call is now non-blocking. However, if it fails, it is not retried.

### Adding retries

```go
go func() {
    for {
        if err := AddToNewsletter(u); err != nil {
            log.Printf("failed to add user to newsletter: %v", err)
            time.Sleep(1 * time.Second)
            continue
        }
        break
    }
}()
```

This retries indefinitely with a 1-second sleep between attempts, until the call succeeds.

### Resulting sign-up function

```go
func SignUp(u User) error {
    // Critical — stays synchronous
    if err := CreateUserAccount(u); err != nil {
        return err
    }

    // Non-critical — run async with retries
    go func() {
        for {
            if err := AddToNewsletter(u); err != nil {
                log.Printf("failed to add user to newsletter: %v", err)
                time.Sleep(1 * time.Second)
                continue
            }
            break
        }
    }()

    go func() {
        for {
            if err := SendNotification(u); err != nil {
                log.Printf("failed to send notification: %v", err)
                time.Sleep(1 * time.Second)
                continue
            }
            break
        }
    }()

    return nil
}
```

**Key takeaway:** Separate critical operations (must succeed now) from non-critical operations (must eventually succeed). Use async patterns for the latter to improve resilience and user experience.
