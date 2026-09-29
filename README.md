# myCart

## How to Run

### 1. Build from Source (Linux)

```bash
# Clone
git clone https://github.com/dure-one/dure-mycart.git
cd dure-mycart

# Build
go build -o mycart

# Initialize
./mycart init

# Run
./mycart serve
```

Access at http://localhost:8080

### 2. Download Binary

```bash
# Download latest release
curl -L https://github.com/dure-one/dure-mycart/releases/latest/download/mycart-linux-amd64 -o mycart
chmod +x mycart

# Initialize
./mycart init

# Run
./mycart serve
```

### 3. Docker

#### Without Prosody

```bash
docker run -v ./lc_base:/lc_base -v ./lc_digitals:/lc_digitals -v ./lc_uploads:/lc_uploads --rm shurco/mycart:latest init

docker run --name mycart --restart unless-stopped -p 8080:8080 -v ./lc_base:/lc_base -v ./lc_digitals:/lc_digitals -v ./lc_uploads:/lc_uploads shurco/mycart:latest
```

#### With Prosody (XMPP)

```bash
cd xmpp-proxy-stack
docker compose up -d
```

Image: `ghcr.io/dure-one/dure-mycart-prosody:0.0`

---

## First-time Setup

Visit http://localhost:8080/_/install or run:

```bash
./mycart install --email admin@example.com --password yourpass --domain localhost
```
