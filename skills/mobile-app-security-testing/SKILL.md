---
name: mobile-app-security-testing
description: Professional skills and methodology for mobile application security testing
version: 1.0.0
---

# Mobile Application Security Testing

## Overview

Mobile application security testing helps secure mobile apps. This skill provides methods, tools, and best practices for Android and iOS.

## Testing Scope

### 1. Application Security

**Items to check:**
- Code obfuscation
- Decompilation protection
- Debugging protection
- Certificate pinning

### 2. Data Security

**Items to check:**
- Data encryption
- Key management
- Sensitive data storage
- Data transmission

### 3. Authentication and Authorization

**Items to check:**
- Authentication mechanisms
- Token management
- Biometrics
- Session management

### 4. Communication Security

**Items to check:**
- TLS/SSL configuration
- Certificate validation
- API security
- Man-in-the-middle protection

## Android Security Testing

### Static Analysis

**Using APKTool:**
```bash
# Decompile the APK
apktool d app.apk

# View AndroidManifest.xml
cat app/AndroidManifest.xml

# View Smali code
find app/smali -name "*.smali"
```

**Using Jadx:**
```bash
# Decompile the APK
jadx -d output app.apk

# View Java source
find output -name "*.java"
```

**Using MobSF:**
```bash
# Start MobSF
docker run -it -p 8000:8000 opensecurity/mobsf

# Upload an APK for analysis
# Open http://localhost:8000
```

### Dynamic Analysis

**Using Frida:**
```javascript
// Hook a function
Java.perform(function() {
    var MainActivity = Java.use("com.example.MainActivity");
    MainActivity.onCreate.implementation = function(savedInstanceState) {
        console.log("[*] onCreate called");
        this.onCreate(savedInstanceState);
    };
});
```

**Using Objection:**
```bash
# Start Objection
objection -g com.example.app explore

# Hook a function
android hooking watch class_method com.example.MainActivity.onCreate
```

**Using Burp Suite:**
```bash
# Configure the proxy
# Set the Android proxy to Burp Suite
# Install the Burp certificate
```

### Common Vulnerabilities

**Hardcoded keys:**
```java
// Insecure code
String apiKey = "1234567890abcdef";
String password = "admin123";
```

**Insecure storage:**
```java
// Sensitive data stored in SharedPreferences
SharedPreferences prefs = getSharedPreferences("data", MODE_WORLD_READABLE);
prefs.edit().putString("password", password).apply();
```

**Certificate validation bypass:**
```java
// No certificate validation
TrustManager[] trustAllCerts = new TrustManager[] {
    new X509TrustManager() {
        public X509Certificate[] getAcceptedIssuers() { return null; }
        public void checkClientTrusted(X509Certificate[] certs, String authType) { }
        public void checkServerTrusted(X509Certificate[] certs, String authType) { }
    }
};
```

## iOS Security Testing

### Static Analysis

**Using class-dump:**
```bash
# Export header files
class-dump app.ipa

# View header files
find app -name "*.h"
```

**Using Hopper:**
```bash
# Disassemble with Hopper
# Open the app binary
# Analyze assembly code
```

**Using otool:**
```bash
# View Mach-O information
otool -L app

# View strings
strings app | grep -i "password\|key\|secret"
```

### Dynamic Analysis

**Using Frida:**
```javascript
// Hook an Objective-C method
var className = ObjC.classes.ViewController;
var method = className['- login:password:'];
Interceptor.attach(method.implementation, {
    onEnter: function(args) {
        console.log("[*] Login called");
        console.log("Username: " + ObjC.Object(args[2]).toString());
        console.log("Password: " + ObjC.Object(args[3]).toString());
    }
});
```

**Using Cycript:**
```bash
# Attach to the process
cycript -p app

# Execute commands
[UIApplication sharedApplication]
```

### Common Vulnerabilities

**Hardcoded keys:**
```objective-c
// Insecure code
NSString *apiKey = @"1234567890abcdef";
NSString *password = @"admin123";
```

**Insecure storage:**
```objective-c
// Improper Keychain storage
NSUserDefaults *defaults = [NSUserDefaults standardUserDefaults];
[defaults setObject:password forKey:@"password"];
```

**Certificate validation bypass:**
```objective-c
// No certificate validation
- (void)connection:(NSURLConnection *)connection 
didReceiveAuthenticationChallenge:(NSURLAuthenticationChallenge *)challenge {
    [challenge.sender useCredential:[NSURLCredential credentialForTrust:challenge.protectionSpace.serverTrust] 
          forAuthenticationChallenge:challenge];
}
```

## Tools

### MobSF

```bash
# Start MobSF
docker run -it -p 8000:8000 opensecurity/mobsf

# Upload an application for analysis
# Supports Android and iOS
```

### Frida

```bash
# Install Frida
pip install frida-tools

# Run the script
frida -U -f com.example.app -l script.js
```

### Objection

```bash
# Install Objection
pip install objection

# Start Objection
objection -g com.example.app explore
```

### Burp Suite

**Configure a proxy:**
1. Configure the Burp Suite listener
2. Set the proxy on the mobile device
3. Install the Burp certificate
4. Intercept and analyze traffic

## Testing Checklist

### Application Security
- [ ] Check code obfuscation
- [ ] Decompilation protection
- [ ] Debugging protection
- [ ] Certificate pinning

### Data Security
- [ ] Check data encryption
- [ ] Key management
- [ ] Sensitive data storage
- [ ] Secure data transmission

### Authentication and Authorization
- [ ] Test authentication mechanisms
- [ ] Token management
- [ ] Session management
- [ ] Biometrics

### Communication Security
- [ ] TLS/SSL configuration
- [ ] Certificate validation
- [ ] API security testing
- [ ] Man-in-the-middle protection

## Common Security Issues

### 1. Hardcoded Keys

**Problem:**
- Hardcoded API keys
- Hardcoded passwords
- Hardcoded encryption keys

**Remediation:**
- Use a key management service
- Use environment variables
- Use secure storage

### 2. Insecure Storage

**Problem:**
- Sensitive data stored in plaintext
- Insecure storage methods
- Unencrypted data

**Remediation:**
- Use encrypted storage
- Use Keychain/Keystore
- Implement data encryption

### 3. Certificate Validation Bypass

**Problem:**
- SSL certificates not validated
- Self-signed certificates accepted
- Certificate pinning not implemented

**Remediation:**
- Implement certificate pinning
- Validate certificate chains
- Use the system certificate store

### 4. Debug Information Disclosure

**Problem:**
- Logs contain sensitive information
- Error message disclosure
- Debug mode not disabled

**Remediation:**
- Remove debug code
- Limit log output
- Disable debugging in production

## Best Practices

### 1. Code Security

- Implement code obfuscation
- Disable debugging features
- Implement anti-debugging protection
- Use certificate pinning

### 2. Data Security

- Encrypt sensitive data
- Use secure storage
- Implement key management
- Restrict data access

### 3. Communication Security

- Use TLS/SSL
- Implement certificate pinning
- Validate server certificates
- Use secure APIs

### 4. Authentication Security

- Implement strong authentication
- Manage tokens securely
- Implement session management
- Use biometrics

## Considerations

- Test only in authorized environments
- Follow applicable laws and regulations
- Account for platform differences
- Protect user privacy