# Creating Your Own Config Wizard Template

This guide is for **VPN providers and service administrators** who want to create a custom configuration template (`config_template.json`) that users can configure through the built-in wizard interface.

---

## What is the Config Wizard?

The Config Wizard is a visual interface that helps users create their `config.json` file without manually editing JSON. As a provider, you create a template file (`config_template.json`) that defines:

- Default settings (DNS, logging, routing rules)
- Which rules users can enable/disable via checkboxes
- How generated proxy outbounds are inserted
- Default proxy groups and selectors

Users simply:
1. Enter their subscription URL or direct proxy links
2. Choose which optional rules to enable
3. Click "Save" to generate their `config.json`

---

## Quick Start

### Step 1: Create the Template File

Create a file named `config_template.json` in the `bin/` folder (next to the application executable).

### Step 2: Use This Minimal Template

Copy this skeleton and customize it:

```jsonc
{
  /** @ParcerConfig
    {
      "ParserConfig": {
        "version": 2,
        "proxies": [
          {
            "source": "https://your-subscription-url-here"
          }
        ],
        "outbounds": [
          {
            "tag": "auto-proxy-out",
            "type": "urltest",
            "options": {
              "url": "https://cp.cloudflare.com/generate_204",
              "interval": "5m",
              "tolerance": 100
            },
            "outbounds": {
              "proxies": {}
            }
          },
          {
            "tag": "proxy-out",
            "type": "selector",
            "options": {
              "default": "auto-proxy-out"
            },
            "outbounds": {
              "addOutbounds": ["direct-out", "auto-proxy-out"],
              "proxies": {}
            }
          }
        ]
      }
    }
  */

  "log": {
    "level": "info",
    "timestamp": true
  },

  "dns": {
    "servers": [
      {
        "type": "udp",
        "tag": "direct_dns",
        "server": "9.9.9.9",
        "server_port": 53
      }
    ],
    "final": "direct_dns"
  },

  "inbounds": [
    {
      "type": "tun",
      "tag": "tun-in",
      "interface_name": "singbox-tun0",
      "address": ["172.16.0.1/30"],
      "mtu": 1400,
      "auto_route": true,
      "strict_route": false,
      "stack": "system"
    }
  ],

  "outbounds": [
    /** @PARSER_OUTBOUNDS_BLOCK */
    { "type": "direct", "tag": "direct-out" }
  ],

  "route": {
    "rules": [
      { "ip_is_private": true, "outbound": "direct-out" },
      { "domain_suffix": ["local"], "outbound": "direct-out" },
      
      /** @SelectableRule
        @label Block Ads
        @description Soft-block ads by rejecting connections
        @default
        { "rule_set": "ads-all", "action": "reject" },
      */
      
      /** @SelectableRule
        @label Route Russian domains directly
        @description Send Russian domain traffic directly
        { "rule_set": "ru-domains", "outbound": "direct-out" },
      */
    ],
    "final": "proxy-out"
  },

  "experimental": {
    "clash_api": {
      "external_controller": "127.0.0.1:9090",
      "secret": "CHANGE_THIS_TO_YOUR_SECRET_TOKEN"
    }
  }
}
```

### Step 3: Customize for Your Service

1. **Update `@ParcerConfig`**: Replace `"source": "https://your-subscription-url-here"` with your actual subscription URL (or leave empty if users will enter their own)
2. **Adjust outbound groups**: Modify the `outbounds` array in `@ParcerConfig` to match your proxy group structure
3. **Add your rules**: Replace the example `@SelectableRule` blocks with your own routing rules
4. **Set default final**: Change `"final": "proxy-out"` to match your default proxy group tag

---

## Understanding the Special Markers

### 1. `@ParcerConfig` Block

**Purpose**: Defines the default subscription parser configuration.

**Location**: Must be a block comment `/** @ParcerConfig ... */` at the top of your template.

**What it contains**:
- `version`: Always use `2`
- `proxies`: Array with subscription URLs (users can override these in the wizard)
- `outbounds`: Default proxy groups (urltest, selector, etc.) that will be available to users

**Example**:
```jsonc
/** @ParcerConfig
  {
    "ParserConfig": {
      "version": 2,
      "proxies": [{ "source": "https://example.com/subscription" }],
      "outbounds": [
        {
          "tag": "auto-proxy-out",
          "type": "urltest",
          "options": { "url": "https://cp.cloudflare.com/generate_204", "interval": "5m" },
          "outbounds": { "proxies": {} }
        }
      ]
    }
  }
*/
```

**Important**: The wizard normalizes this to version 2 format automatically. Make sure all `tag` values match what you reference in your `route` section.

---

### 2. `@PARSER_OUTBOUNDS_BLOCK` Marker

**Purpose**: Tells the wizard where to insert generated proxy outbounds from the user's subscription.

**Location**: Must be inside the `outbounds` array as a comment.

**How it works**:
- The wizard parses the user's subscription URL or direct links
- It generates individual proxy outbounds (vless://, vmess://, etc.)
- These are inserted at the location of `@PARSER_OUTBOUNDS_BLOCK`
- Any outbounds **after** this marker are preserved (like `direct-out`)

**Example**:
```jsonc
"outbounds": [
  /** @PARSER_OUTBOUNDS_BLOCK */
  { "type": "direct", "tag": "direct-out" },
  { "type": "block", "tag": "block-out" }
]
```

**Result**: Generated proxies appear first, then `direct-out`, then `block-out`.

---

### 3. `@SelectableRule` Blocks

**Purpose**: Creates user-friendly checkboxes for optional routing rules.

**Location**: Must be inside the `route.rules` array as block comments.

**Structure**:
```jsonc
/** @SelectableRule
  @label Display Name
  @description Detailed explanation shown in tooltip
  @default
  { "rule_set": "example", "outbound": "proxy-out" },
*/
```

**Directives** (all optional):
- `@label Text` - The checkbox label (required for clarity)
- `@description Text` - Shown when user clicks "?" button
- `@default` - Rule is checked/enabled by default

**Rule format**: Can be a single rule object or an array of rules:
```jsonc
// Single rule
/** @SelectableRule
  @label Block ads
  { "rule_set": "ads-all", "action": "reject" },
*/

// Multiple rules (array)
/** @SelectableRule
  @label Gaming rules
  [
    { "rule_set": "games", "outbound": "direct-out" },
    { "port": [27000, 27001], "outbound": "direct-out" }
  ],
*/
```

**Outbound selection**: If your rule has an `outbound` field, the wizard automatically shows a dropdown so users can choose which proxy group to use. Available options come from:
- Tags defined in `@ParcerConfig` outbounds
- Tags from generated proxies
- Always available: `direct-out`, `reject`, `drop`

**Special case - reject rules**: If your rule has `"action": "reject"`, the wizard offers `reject` and `drop` options instead of proxy groups.

---

## Example Rules with Detailed Explanations

This section provides detailed examples of `@SelectableRule` blocks with explanations of how they work in the wizard.

### Example 1: Rule with Outbound Selection

```jsonc
/** @SelectableRule
    @label Gemini via Gemini VPN
    @default
    @description Use dedicated Gemini VPN selector for Gemini rule set.
    { "rule_set": "gemini", "network": ["tcp", "udp"], "outbound": "proxy-out" },
*/
```

**How it works:**
- This rule has an `outbound` field (`"outbound": "proxy-out"`), so the wizard will show a dropdown selector
- Users can choose from:
  - Any outbound tag defined in `@ParcerConfig` (e.g., `proxy-out`, `auto-proxy-out`)
  - Any generated proxy tag from subscriptions (e.g., `🇳🇱Netherlands`, `🇺🇸USA`)
  - Always available options: `direct-out`, `reject`
- The `@default` directive means this rule is checked/enabled by default when the wizard opens
- The `@description` provides helpful context in the tooltip

### Example 2: Rule with Direct Outbound Default

```jsonc
/** @SelectableRule
    @label Games direct
    @default
    @description Send gaming rule set traffic directly for lower latency.
    { "rule_set": "games", "network": ["tcp", "udp"], "outbound": "direct-out" },
*/
```

**How it works:**
- Uses `direct-out` by default (traffic bypasses VPN)
- Users can still change it in the wizard to route gaming traffic through VPN if needed
- The `network` field specifies both TCP and UDP protocols (gaming often uses UDP)
- Marked with `@default` because low-latency direct routing is usually preferred for gaming

### Example 3: Blocking Rule with Reject Option

```jsonc
/** @SelectableRule
    @label Block ads
    @description Block advertising domains.
    { "rule_set": "ads", "action": "reject", "method": "drop" },
*/
```

**How it works:**
- This rule uses `"action": "reject"` to block ads by default
- The wizard automatically shows an "Outbound:" dropdown with **all available options**:
  - `reject` - Soft block (rejects connections) - selected by default
  - `drop` - Hard block (drops packets)
  - `direct-out` - Traffic goes directly, bypassing VPN (not recommended for ads)
  - Any proxy groups - Route through VPN (usually not needed)
- **Important:** Users can always change the rule behavior in the wizard by selecting any option from the list, regardless of what's specified in the template

**Rule Requirements:**
- All `@SelectableRule` blocks **must** have either an `outbound` field or an `action: reject` field
- Rules without these fields are not supported by the wizard and will not appear in the interface
- Rules with `action: resolve`, `action: sniff`, and other actions without `outbound` or `action: reject` should be placed in the template as regular rules (without `@SelectableRule`)

---

## DNS Configuration and Traffic Splitting

This section explains how DNS configuration works in templates and how to route DNS queries through different paths.

### Understanding DNS Servers Structure

In the example template, DNS servers are configured as follows:

```jsonc
"dns": {
  "servers": [
    {
      "type": "udp",
      "tag": "direct_dns_resolver",
      "server": "9.9.9.9",
      "server_port": 53
    },
    {
      "type": "https",
      "tag": "google_doh",
      "server": "8.8.8.8",
      "server_port": 443,
      "path": "/dns-query"
    },
    {
      "type": "https",
      "tag": "google_doh_vpn",
      "server": "8.8.8.8",
      "server_port": 443,
      "path": "/dns-query",
      "detour": "proxy-out"
    },
    {
      "type": "https",
      "tag": "yandex_doh",
      "server": "77.88.8.88",
      "server_port": 443,
      "path": "/dns-query",
      "domain_strategy": "prefer_ipv4"
    },
    {
      "type": "udp",
      "tag": "yandex_dns_vpn",
      "server": "77.88.8.2",
      "server_port": 53,
      "detour": "proxy-out",
      "domain_strategy": "prefer_ipv4"
    },
    {
      "type": "udp",
      "tag": "yandex_dns_direct",
      "server": "77.88.8.2",
      "server_port": 53,
      "domain_strategy": "prefer_ipv4"
    }
  ]
}
```

**DNS Server Types:**

1. **`direct_dns_resolver`** - Standard UDP DNS (9.9.9.9 - Quad9)
   - Direct connection, no encryption
   - Fast and reliable fallback

2. **`google_doh`** - DNS-over-HTTPS (8.8.8.8)
   - Encrypted DNS queries
   - Direct connection (not through VPN)
   - Good for privacy and bypassing DNS censorship

3. **`google_doh_vpn`** - DNS-over-HTTPS through VPN
   - Same as `google_doh` but routed through `proxy-out` (via `detour`)
   - Use when you want DNS queries to appear from VPN location
   - Helps bypass DNS-based geo-blocking

4. **`yandex_doh`** - Yandex DNS-over-HTTPS
   - Direct connection
   - Useful for resolving Russian domains correctly

5. **`yandex_dns_vpn`** - Yandex DNS through VPN
   - Routes through `proxy-out` via `detour`
   - Use for Russian domains when VPN is active

6. **`yandex_dns_direct`** - Yandex DNS direct UDP
   - Fast UDP queries for Russian domains
   - Direct connection

### DNS Rules for Traffic Splitting

DNS rules determine which DNS server to use for different domains:

```jsonc
"rules": [
  { "rule_set": "ru-domains", "server": "yandex_doh" },
  { "rule_set": "ru-domains", "server": "yandex_dns_vpn" },
  { "rule_set": "ru-domains", "server": "yandex_dns_direct" },
  { "server": "google_doh" },
  { "server": "google_doh_vpn" }
],
"final": "direct_dns_resolver"
```

**How it works:**
- Rules are processed **top to bottom** (first match wins)
- `rule_set: "ru-domains"` matches Russian domains (`.ru`, `.рф`)
- For Russian domains, it tries `yandex_doh`, then `yandex_dns_vpn`, then `yandex_dns_direct`
- For all other domains, uses `google_doh`, then falls back to `google_doh_vpn`
- `final` is the ultimate fallback if no rules match (uses `direct_dns_resolver`)

**When to use DNS through VPN:**
- ✅ You want DNS queries to appear from VPN location
- ✅ You need to bypass DNS-based geo-restrictions
- ✅ Your ISP blocks certain DNS servers
- ✅ You want enhanced privacy for DNS queries

**When to use direct DNS:**
- ✅ Faster response times (no VPN overhead)
- ✅ Resolving local domains correctly (e.g., `.local`, `.lan`)
- ✅ Accessing services that require local DNS resolution
- ✅ Reducing VPN load for DNS queries

### Important DNS Settings

**`domain_strategy: "prefer_ipv4"`:**
- Forces IPv4 resolution even if IPv6 is available
- Prevents connection issues with services that prefer IPv4
- Recommended for better compatibility

**`strategy: "ipv4_only"`** (at DNS level):
- Only resolves to IPv4 addresses
- Useful if your network or VPN doesn't support IPv6
- Reduces DNS resolution overhead

**`independent_cache: false`:**
- DNS cache is shared across all DNS servers
- When you switch VPN servers, cached DNS results are still valid
- Set to `true` if you want separate cache per DNS server (usually not needed)

---

## TUN vs System Proxy (SOCKS/HTTP)

This section explains the difference between TUN mode and system proxy mode, and when to use each.

### TUN Mode (Virtual Network Interface)

**What it is:**
TUN creates a virtual network interface that captures **all traffic** from your system.

**Example configuration:**
```jsonc
{
  "type": "tun",
  "tag": "tun-in",
  "interface_name": "singbox-tun0",
  "address": ["172.16.0.1/30"],
  "mtu": 1400,
  "auto_route": true,
  "strict_route": false,
  "stack": "system"
}
```

**TUN Parameter Description:**

- **`type: "tun"`** - Interface type. **DO NOT CHANGE** - must always be `"tun"`.

- **`tag: "tun-in"`** - Tag for referencing in routing rules.
  - ✅ **Can be changed** to any unique name (e.g., `"my-tun"`)
  - ⚠️ **Important:** If changed, update all references to this tag in rules (`{ "inbound": "tun-in" }`)

- **`interface_name: "singbox-tun0"`** - Network interface name in the system.
  - ✅ **Can be changed** to any unique name (e.g., `"my-vpn"`)
  - ⚠️ **Important:** On Linux must start with `tun` (e.g., `tun0`, `tun1`)
  - 💡 Recommended to leave default to avoid conflicts

- **`address: ["172.16.0.1/30"]`** - IP address and subnet mask for the TUN interface.
  - ✅ **Can be changed** to another private range (e.g., `["10.0.0.1/30"]`, `["192.168.100.1/30"]`)
  - ⚠️ **Important:** Use only private ranges (RFC 1918):
    - `10.0.0.0/8` - `10.0.0.0` to `10.255.255.255`
    - `172.16.0.0/12` - `172.16.0.0` to `172.31.255.255`
    - `192.168.0.0/16` - `192.168.0.0` to `192.168.255.255`
  - `/30` means 4 addresses (usually sufficient)
  - 💡 If changed, ensure it doesn't conflict with your local network

- **`mtu: 1400`** - Maximum Transmission Unit (maximum packet size).
  - ✅ **Can be changed** in range `1280-1500` (recommended `1300-1450`)
  - 💡 Smaller MTU helps avoid packet fragmentation:
    - `1400` - universal option (good for most cases)
    - `1300` - if experiencing fragmentation issues
    - `1280` - minimum for IPv6, if compatibility needed
  - ❌ Don't set larger than `1500` (standard Ethernet MTU)

- **`auto_route: true`** - Automatically add routes for all traffic.
  - ❌ **DO NOT CHANGE** - must be `true` for full VPN functionality
  - If `false`, TUN will not intercept system traffic

- **`strict_route: false`** - Strict routing (force all traffic through TUN).
  - ✅ **Can be changed** to `true` in some cases:
    - `false` (default) - allows route bypass through rules
    - `true` - forcefully sends all traffic through TUN (stricter mode)
  - 💡 Recommended to leave `false` for more flexibility

- **`stack: "system"`** - Network stack for packet processing.
  - ❌ **DO NOT CHANGE** without necessity
  - `"system"` - uses system stack (recommended)
  - `"gvisor"` - uses gVisor (experimental, for special cases)
  - 💡 Leave `"system"` for stable operation

**Recommendations:**
- For most cases, leave all parameters as in the example
- If changing `tag` or `interface_name`, ensure uniqueness
- Change `address` only if it conflicts with local network
- Adjust `mtu` only if experiencing packet fragmentation issues

**When to use TUN:**
- ✅ You want **full system-wide VPN** (all applications automatically)
- ✅ You have administrator/root privileges
- ✅ You need transparent proxy for all traffic
- ✅ You want to protect all network activity
- ✅ You need to route traffic that doesn't support proxy settings

**Advantages:**
- ✅ Works with **all applications** (browsers, games, apps without proxy support)
- ✅ Transparent - applications don't need proxy configuration
- ✅ Captures all traffic automatically
- ✅ Best for full VPN experience

**Disadvantages:**
- ❌ Requires administrator/root privileges
- ❌ More complex setup
- ❌ May interfere with some network services
- ❌ Need proper routing rules to avoid breaking local network

### System Proxy (SOCKS/HTTP)

**What it is:**
System proxy mode runs a proxy server (SOCKS or HTTP) that applications connect to manually.

**SOCKS Proxy Example:**
```jsonc
{
  "type": "socks",
  "tag": "socks-in",
  "listen": "127.0.0.1",
  "listen_port": 1080
}
```

**HTTP Proxy Example:**
```jsonc
{
  "type": "http",
  "tag": "http-in",
  "listen": "127.0.0.1",
  "listen_port": 8080
}
```

**When to use System Proxy:**
- ✅ You don't have administrator/root privileges
- ✅ You want **selective proxying** (only specific applications)
- ✅ You need applications to explicitly use proxy
- ✅ You need **separate proxy and different settings for some applications** (different apps use different proxy servers)
- ✅ You want easier setup without TUN driver installation
- ✅ You're testing or need quick proxy setup

**Advantages:**
- ✅ No admin privileges required
- ✅ Selective - choose which apps use proxy
- ✅ Easier to set up and configure
- ✅ Can run multiple proxy instances on different ports
- ✅ Works on systems where TUN is not available

**Disadvantages:**
- ❌ Not all applications support proxy settings
- ❌ Need to configure each application manually
- ❌ Some applications ignore system proxy settings
- ❌ Less transparent - applications must explicitly support proxy
- ❌ DNS queries may not be routed through proxy (unless configured)

### Comparison Table

| Feature | TUN Mode | System Proxy |
|---------|----------|--------------|
| **Scope** | All system traffic | Selected applications only |
| **Setup Complexity** | Higher | Lower |
| **Admin Rights** | Required | Not required |
| **Application Support** | All apps (automatic) | Apps with proxy support only |
| **Transparency** | Full | Partial |
| **DNS Routing** | Automatic | Manual configuration needed |
| **Network Isolation** | Complete | Application-dependent |
| **Best For** | Full VPN experience | Selective proxying |

### Recommendation

- **For most users**: Use **TUN mode** for full VPN protection
- **For developers/testing**: Use **system proxy** for quick setup and testing
- **For restricted environments**: Use **system proxy** if admin rights unavailable

---

## Local Traffic Rules - Critical for Home Networks

This section explains why local traffic rules are essential and what happens without them.

### The Problem

Without proper local traffic rules, VPN users often experience:
- ❌ Cannot access router web interface (192.168.1.1, 192.168.0.1)
- ❌ Cannot access local network devices (NAS, printers, smart home devices)
- ❌ Local domain names don't work (`.local`, `.lan`)
- ❌ Home network file sharing breaks (SMB, DLNA)
- ❌ Smart home devices become unreachable
- ❌ Cannot configure network devices through web interface

### The Solution: Local Traffic Rules

Always include these critical rules in your template:

```jsonc
"rules": [
  { "ip_is_private": true, "outbound": "direct-out" },
  { "domain_suffix": ["local"], "outbound": "direct-out" }
]
```

### Understanding the Rules

#### Rule 1: Private IP Addresses

```jsonc
{ "ip_is_private": true, "outbound": "direct-out" }
```

**What it covers:**
- All private IPv4 ranges:
  - `192.168.0.0/16` (192.168.0.0 - 192.168.255.255)
  - `10.0.0.0/8` (10.0.0.0 - 10.255.255.255)
  - `172.16.0.0/12` (172.16.0.0 - 172.31.255.255)
- Router interfaces (e.g., 192.168.1.1, 192.168.0.1)
- Local network devices (NAS, printers, IP cameras)
- Other devices on your home network

**Why it's critical:**
- Without this rule, VPN tries to route local IPs through VPN server
- VPN servers typically don't have routes to your local network
- Result: **Cannot access anything on your local network**

#### Rule 2: Local Domain Suffixes

```jsonc
{ "domain_suffix": ["local"], "outbound": "direct-out" }
```

**What it covers:**
- Domain names ending in `.local` (e.g., `printer.local`, `nas.local`)
- Domain names ending in `.lan` (if added: `["local", "lan"]`)
- mDNS (multicast DNS) resolved names
- Local hostnames discovered via network discovery

**Why it's critical:**
- Many devices use `.local` domains for automatic discovery
- Without this rule, `.local` domains may try to resolve through VPN DNS
- Result: **Local devices become unreachable by name**

### What Happens Without These Rules

**Scenario: User tries to access router (192.168.1.1)**

**Without local traffic rules:**
1. Request goes to VPN server
2. VPN server doesn't have route to 192.168.1.1
3. Connection fails or times out
4. ❌ **User cannot configure router**

**With local traffic rules:**
1. Rule matches `ip_is_private: true`
2. Traffic routed to `direct-out` (bypasses VPN)
3. Request goes directly to router on local network
4. ✅ **Router accessible**

**Scenario: User tries to access NAS (nas.local)**

**Without local traffic rules:**
1. DNS query for `nas.local` may go through VPN DNS
2. VPN DNS doesn't know about local `.local` domains
3. Resolution fails
4. ❌ **Cannot access NAS**

**With local traffic rules:**
1. Rule matches `domain_suffix: ["local"]`
2. Traffic routed to `direct-out`
3. DNS resolves via local mDNS
4. ✅ **NAS accessible**

### Additional Local Network Considerations

#### Including `.lan` Domain Suffix

If your network uses `.lan` domains, add them:

```jsonc
{ "domain_suffix": ["local", "lan"], "outbound": "direct-out" }
```

#### Placement in Rules Array

**Important:** Local traffic rules should be placed **early** in the rules array (after critical system rules but before geo-routing rules):

```jsonc
"rules": [
  { "inbound": "tun-in", "action": "resolve", "strategy": "prefer_ipv4" },
  { "inbound": "tun-in", "action": "sniff", "timeout": "1s" },
  { "protocol": "dns", "action": "hijack-dns" },
  
  // Local traffic rules - CRITICAL - place early
  { "ip_is_private": true, "outbound": "direct-out" },
  { "domain_suffix": ["local"], "outbound": "direct-out" },
  
  // Other rules...
  { "rule_set": "ads-all", "action": "reject" },
  { "rule_set": "ru-domains", "outbound": "direct-out" },
  // ...
]
```

This ensures local traffic is matched **before** other rules that might interfere.

### Understanding System Action Rules

At the beginning of the rules array, you typically place system rules with special actions that don't route traffic but perform preprocessing:

#### `action: resolve` - DNS Resolution

```jsonc
{ "inbound": "tun-in", "action": "resolve", "strategy": "prefer_ipv4" }
```

**What it does:**
- Performs DNS resolution (converts domain names to IP addresses) for traffic going through the TUN interface
- The `strategy: "prefer_ipv4"` parameter prefers IPv4 addresses even if IPv6 is available
- This allows sing-box to know the actual destination IP addresses **before** applying routing rules

**Why it's needed:**
- Without this rule, sing-box may not know IP addresses, which complicates routing
- Required for proper operation of other rules that work with IP addresses
- Improves compatibility and stability of VPN operation

#### `action: sniff` - Protocol Detection and Metadata Extraction

```jsonc
{ "inbound": "tun-in", "action": "sniff", "timeout": "1s" }
```

**What it does:**
- Analyzes initial bytes of a connection and determines the protocol (HTTP, TLS, DNS, BitTorrent, etc.)
- Extracts metadata from traffic, for example:
  - **SNI (Server Name Indication)** from TLS handshake - the actual domain name even when connecting by IP address
  - Protocol type for proper routing
- The `timeout: "1s"` parameter limits analysis time - if the protocol is not determined within 1 second, the connection is processed without sniffing

**Why it's needed:**
- Allows using domain-based routing rules (`domain`, `domain_suffix`, `domain_keyword`) even when applications connect by IP address
- Without sniffing, sing-box only sees IP addresses in TUN mode, which limits routing capabilities
- Critically important for proper operation of domain-based rules

**Example:** If an application connects to `1.2.3.4`, but that's the IP for `google.com`, sniffing will extract the domain from the TLS handshake, and rules for `google.com` will work correctly.

#### `action: hijack-dns` - DNS Query Hijacking

```jsonc
{ "protocol": "dns", "action": "hijack-dns" }
```

**What it does:**
- Intercepts DNS queries and routes them to DNS servers configured in the `dns` section
- Forces all DNS queries to use the configured DNS setup instead of system DNS
- The rule only triggers for traffic with the `dns` protocol

**Why it's needed:**
- Ensures all DNS queries are processed through configured DNS servers (DoH, DoT, specific servers for different domains)
- Without this rule, applications may use system DNS, bypassing your configuration
- Allows applying DNS rules from the `dns.rules` section for traffic splitting

**Important:** This rule should be placed **before** local traffic rules to ensure DNS queries are processed correctly.

### Recommendations for VPN Providers

⚠️ **CRITICAL:** Always include local traffic rules in your templates

1. **Never skip these rules** - Users will have broken home networks without them
2. **Place them early** in the rules array for reliable matching
3. **Document them** - Explain to users why these rules are important
4. **Test thoroughly** - Verify router access and local device connectivity

**Example template snippet:**
```jsonc
"rules": [
  // System rules
  { "inbound": "tun-in", "action": "resolve", "strategy": "prefer_ipv4" },
  { "protocol": "dns", "action": "hijack-dns" },
  
  // CRITICAL: Local network access - DO NOT REMOVE
  { "ip_is_private": true, "outbound": "direct-out" },
  { "domain_suffix": ["local"], "outbound": "direct-out" },
  
  // User-selectable rules
  /** @SelectableRule ... */
]
```

### Troubleshooting Local Network Issues

If users report local network access problems:

1. **Verify rules exist** - Check that both rules are in the config
2. **Check rule order** - Ensure local rules are before geo-routing rules
3. **Test router access** - Try accessing 192.168.1.1 or 192.168.0.1
4. **Check DNS** - Verify `.local` domains resolve correctly
5. **Review logs** - Check sing-box logs for routing decisions

---

## Complete Example: Real-World Template

Here's a more complete example showing best practices:

```jsonc
{
  /** @ParcerConfig
    {
      "ParserConfig": {
        "version": 2,
        "proxies": [
          {
            "source": "https://your-vpn-service.com/api/subscription?token=USER_TOKEN"
          }
        ],
        "outbounds": [
          {
            "tag": "auto-proxy-out",
            "type": "urltest",
            "options": {
              "url": "https://cp.cloudflare.com/generate_204",
              "interval": "5m",
              "tolerance": 100,
              "interrupt_exist_connections": true
            },
            "outbounds": {
              "proxies": {}
            },
            "comment": "Auto-select fastest proxy"
          },
          {
            "tag": "proxy-out",
            "type": "selector",
            "options": {
              "interrupt_exist_connections": true,
              "default": "auto-proxy-out"
            },
            "outbounds": {
              "addOutbounds": ["direct-out", "auto-proxy-out"],
              "proxies": {}
            },
            "comment": "Main proxy selector"
          }
        ]
      }
    }
  */

  "log": {
    "level": "warn",
    "timestamp": true
  },

  "dns": {
    "servers": [
      {
        "type": "udp",
        "tag": "direct_dns",
        "server": "9.9.9.9",
        "server_port": 53
      },
      {
        "type": "https",
        "tag": "cloudflare_doh",
        "server": "1.1.1.1",
        "server_port": 443,
        "path": "/dns-query"
      }
    ],
    "rules": [
      { "server": "cloudflare_doh" }
    ],
    "final": "direct_dns"
  },

  "inbounds": [
    {
      "type": "tun",
      "tag": "tun-in",
      "interface_name": "singbox-tun0",
      "address": ["172.16.0.1/30"],
      "mtu": 1400,
      "auto_route": true,
      "strict_route": false,
      "stack": "system"
    }
  ],

  "outbounds": [
    /** @PARSER_OUTBOUNDS_BLOCK */
    { "type": "direct", "tag": "direct-out" },
    { "type": "block", "tag": "block-out" }
  ],

  "route": {
    "rule_set": [
      {
        "tag": "ads-all",
        "type": "remote",
        "format": "binary",
        "url": "https://raw.githubusercontent.com/v2fly/domain-list-community/release/geosite-category-ads-all.srs",
        "download_detour": "direct-out",
        "update_interval": "24h"
      },
      {
        "tag": "ru-domains",
        "type": "inline",
        "format": "domain_suffix",
        "rules": [
          { "domain_suffix": ["ru", "xn--p1ai"] }
        ]
      }
    ],
    "rules": [
      { "inbound": "tun-in", "action": "resolve", "strategy": "prefer_ipv4" },
      { "inbound": "tun-in", "action": "sniff", "timeout": "1s" },
      { "protocol": "dns", "action": "hijack-dns" },
      { "ip_is_private": true, "outbound": "direct-out" },
      { "domain_suffix": ["local"], "outbound": "direct-out" },
      
      /** @SelectableRule
        @label Block Ads
        @description Soft-block ads by rejecting connections (recommended)
        @default
        { "rule_set": "ads-all", "action": "reject" },
      */
      
      /** @SelectableRule
        @label Russian domains direct
        @description Route Russian domains directly (faster for local services)
        { "rule_set": "ru-domains", "outbound": "direct-out" },
      */
      
      /** @SelectableRule
        @label Gaming traffic direct
        @description Route gaming traffic directly for lower latency
        @default
        { "rule_set": "games", "outbound": "direct-out" },
      */
    ],
    "final": "proxy-out",
    "auto_detect_interface": true
  },

  "experimental": {
    "clash_api": {
      "external_controller": "127.0.0.1:9090",
      "secret": "CHANGE_THIS_TO_YOUR_SECRET_TOKEN"
    }
  }
}
```

---

## Best Practices

### 1. Always Include Static Outbounds

Keep at least `direct-out` after `@PARSER_OUTBOUNDS_BLOCK`. Users need it for local traffic and as a fallback option.

### 2. Use Clear Labels and Descriptions

Good labels help users understand what each rule does:
- ✅ `@label Block Ads` 
- ❌ `@label Rule 1`

Good descriptions explain the impact:
- ✅ `@description Soft-block ads by rejecting connections instead of dropping packets`
- ❌ `@description Blocks ads`

### 3. Set Sensible Defaults

Use `@default` for rules that most users should enable:
- Ad blocking
- Common geo-routing rules
- Performance optimizations

Don't use `@default` for:
- Experimental features
- Rules that might break common services
- Region-specific rules (unless you're targeting that region)

### 4. Match Tag Names

Ensure all `tag` values referenced in `route.rules` exist in:
- Your `@ParcerConfig` outbounds, OR
- Static outbounds after `@PARSER_OUTBOUNDS_BLOCK`, OR
- Will be generated from subscriptions

### 5. Validate Your JSON

After removing comments, your template must be valid JSON. Test it:
```bash
# Using jq (if installed)
cat config_template.json | jq . > /dev/null

# Or use an online JSON validator
```

### 6. Keep Section Order Logical

The wizard preserves your section order. Organize logically:
1. `log` - Logging settings
2. `dns` - DNS configuration
3. `inbounds` - Incoming connections
4. `outbounds` - Outgoing connections (with `@PARSER_OUTBOUNDS_BLOCK`)
5. `route` - Routing rules
6. `experimental` - Experimental features

---

## Testing Your Template

### Step 1: Place the File

Put `config_template.json` in the `bin/` folder next to the executable.

### Step 2: Launch the Wizard

1. Start the application
2. Open the Config Wizard (usually from the main menu or Tools tab)
3. Verify the template loads without errors

### Step 3: Test User Flow

1. **Tab 1 (Sources & Parser)**:
   - Enter a test subscription URL or direct link
   - Click "Check" - should validate successfully
   - Click "Parse" - should generate outbounds preview

2. **Tab 2 (Rules)**:
   - Verify all your `@SelectableRule` checkboxes appear
   - Check that outbound dropdowns show correct options
   - Toggle some rules on/off

3. **Tab 3 (Preview)**:
   - Click "Show Preview"
   - Verify the generated config looks correct
   - Check that selected rules are included
   - Verify `@PARSER_OUTBOUNDS_BLOCK` was replaced with generated proxies

4. **Save**:
   - Click "Save"
   - Verify `config.json` is created
   - Check that old config was backed up (if existed)
   - Verify `experimental.clash_api.secret` was generated

---

## Troubleshooting

### Template Not Loading

**Problem**: Wizard shows "Template file not found" or JSON errors.

**Solutions**:
- Ensure file is named exactly `config_template.json` (case-sensitive)
- Ensure file is in `bin/` folder
- Validate JSON syntax (remove comments and test)
- Check for trailing commas or syntax errors

### Generated Outbounds Not Appearing

**Problem**: After parsing, no outbounds show in preview.

**Solutions**:
- Verify `@PARSER_OUTBOUNDS_BLOCK` marker exists in `outbounds` array
- Check subscription URL is accessible
- Verify subscription format is supported (vless://, vmess://, trojan://, ss://)
- Check logs in `logs/` folder for parsing errors

### Rules Not Showing in Wizard

**Problem**: `@SelectableRule` blocks don't appear as checkboxes.

**Solutions**:
- Verify `@SelectableRule` is spelled correctly (case-sensitive)
- Ensure block is inside `route.rules` array
- Check that JSON inside block is valid
- Ensure `@label` directive is present

### Outbound Tags Not Found

**Problem**: Wizard shows "outbound not found" errors.

**Solutions**:
- Verify all referenced tags exist in `@ParcerConfig` outbounds
- Ensure `final` tag exists
- Check that generated proxies will have matching tags (if using tag filters)
- Add missing outbounds to static section after `@PARSER_OUTBOUNDS_BLOCK`

---

## Advanced Tips

### Dynamic Subscription URLs

If users need to enter their own subscription URL, leave `source` empty or use a placeholder:
```jsonc
"proxies": [{ "source": "" }]
```

Users will enter their URL in the wizard's first tab.

### Multiple Subscription Sources

Support multiple subscriptions:
```jsonc
"proxies": [
  { "source": "https://provider1.com/subscription" },
  { "source": "https://provider2.com/subscription" }
]
```

Users can add more in the wizard interface.

### Conditional Rules Based on Outbound Selection

When a rule has `outbound`, users can choose from available tags. Make sure your `@ParcerConfig` defines all options users might need.

### Custom Rule Sets

Reference remote rule sets in `route.rule_set`:
```jsonc
"rule_set": [
  {
    "tag": "ads-all",
    "type": "remote",
    "format": "binary",
    "url": "https://example.com/rules/ads.srs",
    "download_detour": "direct-out",
    "update_interval": "24h"
  }
]
```

Then reference them in `@SelectableRule` blocks.

---

## Distribution

When distributing your customized launcher:

1. Include `config_template.json` in your release package
2. Place it in `bin/` folder
3. Users will automatically use it when opening the Config Wizard
4. Consider documenting your specific rules and options in a separate README

---

## Need Help?

- **Template syntax issues**: Check this guide's examples
- **sing-box configuration**: See [official sing-box docs](https://sing-box.sagernet.org/configuration/)
- **ParserConfig format**: See `ParserConfig.md` in this repository
- **Report bugs**: Open an issue on [GitHub](https://github.com/Leadaxe/singbox-launcher/issues)

---

**Note**: This wizard template system is designed to make configuration easier for end users. As a provider, you maintain full control over the default configuration while giving users flexibility to customize their setup.


