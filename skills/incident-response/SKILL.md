---
name: incident-response
description: Professional skills and methodology for security incident response
version: 1.0.0
---

# Security Incident Response

## Overview

Incident response is the process of handling security incidents. This skill provides methods, tools, and best practices.

## Response Process

### 1. Preparation

**Preparation tasks:**
- Establish a response team
- Develop a response plan
- Prepare tools and resources
- Establish communication channels

### 2. Identification

**Identify incidents:**
- Monitoring and alerts
- Anomaly detection
- Log analysis
- User reports

### 3. Containment

**Containment measures:**
- Isolate affected systems
- Disable accounts
- Block network connections
- Back up evidence

### 4. Eradication

**Remove threats:**
- Remove malware
- Remediate vulnerabilities
- Reset credentials
- Remove backdoors

### 5. Recovery

**Restore systems:**
- Restore backups
- Verify system integrity
- Monitor systems
- Restore services gradually

### 6. Lessons Learned

**Review the incident:**
- Incident report
- Lessons learned
- Improvements
- Update procedures

## Tools

### Log Analysis

**Using Splunk:**
```bash
# Search logs
index=security event_type="failed_login"

# Statistical analysis
index=security | stats count by src_ip

# Time-series analysis
index=security | timechart count by event_type
```

**Using ELK:**
```bash
# Elasticsearch query
GET /logs/_search
{
  "query": {
    "match": {
      "event_type": "malware"
    }
  }
}
```

### Forensic Tools

**Using Volatility:**
```bash
# Analyze a memory image
volatility -f memory.dump imageinfo

# List processes
volatility -f memory.dump --profile=Win7SP1x64 pslist

# Extract process memory
volatility -f memory.dump --profile=Win7SP1x64 memdump -p 1234 -D output/
```

**Using Autopsy:**
```bash
# Start Autopsy
# Create a case
# Add evidence
# Analyze data
```

### Network Analysis

**Using Wireshark:**
```bash
# Capture traffic
wireshark -i eth0

# Analyze a PCAP file
wireshark -r capture.pcap

# Filter traffic
# Display filter: ip.addr == 192.168.1.100
# Capture filter: host 192.168.1.100
```

**Using tcpdump:**
```bash
# Capture traffic
tcpdump -i eth0 -w capture.pcap

# Analyze traffic
tcpdump -r capture.pcap -A
```

## Incident Types

### Malware

**Response steps:**
1. Isolate affected systems
2. Collect samples
3. Analyze the malware
4. Remove the threat
5. Remediate vulnerabilities

**Tools:**
- VirusTotal
- Cuckoo Sandbox
- YARA rules

### Data Breach

**Response steps:**
1. Confirm the scope of the breach
2. Contain the breach
3. Assess the impact
4. Notify relevant parties
5. Remediate vulnerabilities

**Items to check:**
- Volume of exposed data
- Affected users
- Disclosure channels
- Data sensitivity

### Denial of Service

**Response steps:**
1. Identify the attack type
2. Enable protective measures
3. Filter malicious traffic
4. Monitor system status
5. Restore normal service

**Protective measures:**
- DDoS protection services
- Traffic scrubbing
- Rate limiting
- CDN protection

### Unauthorized Access

**Response steps:**
1. Disable affected accounts
2. Reset credentials
3. Review access logs
4. Assess data access
5. Remediate vulnerabilities

**Items to check:**
- Access times
- Accessed content
- Access sources
- Data modifications

## Response Checklist

### Preparation
- [ ] Establish a response team
- [ ] Develop a response plan
- [ ] Prepare tools
- [ ] Establish communication channels

### Identification
- [ ] Confirm the incident
- [ ] Collect information
- [ ] Assess the impact
- [ ] Record the timeline

### Containment
- [ ] Isolate systems
- [ ] Disable accounts
- [ ] Block connections
- [ ] Back up evidence

### Eradication
- [ ] Remove threats
- [ ] Remediate vulnerabilities
- [ ] Reset credentials
- [ ] Verify eradication

### Recovery
- [ ] Restore systems
- [ ] Verify integrity
- [ ] Monitor systems
- [ ] Restore services

### Lessons Learned
- [ ] Write the report
- [ ] Review lessons learned
- [ ] Define improvements
- [ ] Update procedures

## Best Practices

### 1. Preparation

- Establish a response team
- Develop a response plan
- Conduct regular exercises
- Prepare tools

### 2. Response

- Respond quickly
- Handle incidents systematically
- Record all actions
- Preserve evidence

### 3. Communication

- Internal communication
- External notification
- Status updates
- Post-incident reporting

### 4. Improvement

- Incident analysis
- Process improvements
- Tool updates
- Training

## Considerations

- Respond quickly
- Preserve evidence
- Record actions
- Follow applicable laws and regulations