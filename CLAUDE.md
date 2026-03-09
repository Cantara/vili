# Vili

## Purpose
A live-data testing and deployment tool that tests new versions of your servers with real production traffic before exposing them to users. Implements a poka-yoke (mistake-proofing) approach to deployments by running two versions simultaneously and comparing their behavior.

## Tech Stack
- Language: Go 1.17
- Framework: None (standalone binary)
- Build: Maven with mvn-golang plugin
- Key dependencies: Go standard library

## Architecture
Reverse proxy that manages two versions of an application simultaneously (running and testing). Routes real traffic to both versions, compares their responses, and automatically promotes the testing version to running if metrics are acceptable. Archives old versions as zip files. Provides symlinks to current running and testing versions for easy reference.

## Key Entry Points
- Main Go source files
- `packages.txt` - Go dependency listing
- `pom.xml` - Maven build configuration (mvn-golang)

## Development
```bash
# Build
mvn clean install

# Run
./target/vili
```

## Domain Context
Deployment safety and canary testing. Provides automated canary deployments by testing new versions against real production traffic, reducing the risk of deploying broken code to users.
