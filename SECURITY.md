# Security Policy

## Supported Versions

We provide security updates for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take security vulnerabilities seriously. If you discover a security vulnerability, please follow these steps:

### For Critical Security Issues

1. **Do NOT create a public GitHub issue**
2. Send an email to security@example.com with:
   - A detailed description of the vulnerability
   - Steps to reproduce the issue
   - Potential impact assessment
   - Any suggested fixes (if available)

### For Non-Critical Security Issues

1. Create a private security advisory through GitHub:
   - Go to the "Security" tab in this repository
   - Click "Report a vulnerability"
   - Fill out the advisory form

### What to Expect

- **Initial Response**: We will acknowledge receipt within 48 hours
- **Assessment**: We will assess the vulnerability within 7 days
- **Updates**: We will provide regular updates every 7 days until resolution
- **Resolution**: We aim to resolve critical vulnerabilities within 30 days

### Responsible Disclosure

We follow responsible disclosure practices:
- We will work with you to understand and resolve the issue
- We will acknowledge your contribution in our security advisories (if desired)
- We ask that you do not publicly disclose the vulnerability until we have had a chance to address it

## Security Best Practices for Contributors

When contributing to this project:

1. **Dependencies**: Keep dependencies up to date and review security advisories
2. **Input Validation**: Always validate user inputs
3. **Error Handling**: Don't expose sensitive information in error messages
4. **Logging**: Be careful not to log sensitive data
5. **Authentication**: Follow secure authentication practices
6. **Permissions**: Use the principle of least privilege

## Security Tools and Processes

This project uses several automated security tools:

- **Dependabot**: Automatically updates dependencies with known vulnerabilities
- **CodeQL**: Static analysis for security vulnerabilities
- **Gosec**: Go-specific security analyzer
- **govulncheck**: Go vulnerability checker

These tools run automatically on pull requests and can be found in our GitHub Actions workflows.

## Security-Related Dependencies

This project aims to minimize dependencies and regularly audits them for security issues. Key security considerations:

- We use only well-maintained, popular Go modules
- Dependencies are pinned to specific versions
- Regular security audits are performed
- Automated dependency updates are enabled through Dependabot

## Incident Response

In case of a security incident:

1. The issue will be immediately triaged by project maintainers
2. A fix will be developed and tested
3. A security advisory will be published
4. Affected users will be notified through GitHub releases and security advisories
5. A post-incident review will be conducted to improve our security posture
