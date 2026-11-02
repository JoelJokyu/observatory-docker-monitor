# Security and deployment scope

Observatory is intended for trusted local development on one Docker Engine. The supplied release binds the dashboard to 127.0.0.1 and does not publish PostgreSQL. There is no application authentication. Do not expose it directly to your network or the Internet.

The app can read Docker metadata and logs across the mounted Engine. Logs may contain private information. Although Observatory uses read APIs, mounting the socket read-only does not make the Docker API read-only; compromising an app with that socket can affect the Engine/host. The container currently runs as root for standard socket access, with other capabilities dropped, a read-only filesystem and no-new-privileges. These settings do not replace Docker API authorization.

The release is pre-1.0, with no formal security audit or guaranteed support window. Keep Docker and dependencies updated. Retained history/settings and backups should be treated as private data. Do not attach .env files, database dumps or unredacted logs to public issues.

## Report a vulnerability privately

Private vulnerability reporting is enabled for this repository. Use [Report a vulnerability](https://github.com/JoelJokyu/observatory-docker-monitor/security/advisories/new) to send security-sensitive details to the maintainer. Include affected versions, impact, reproduction steps and suggested mitigations when available. Do not include live credentials or unrelated private data. Ordinary bugs can use public issues.

## Linux kernel advisory: CVE-2026-98432

Observatory 0.1.0 does not fix or claim immunity to this kernel vulnerability. The application uses Go 1.25 and the standard listener; Go 1.24+ enables MPTCP listeners by default on Linux. Host loopback publication reduces external exposure but does not prove safety of the kernel or container network. Users must follow their Linux/Docker/WSL vendor's patched-kernel guidance. Disabling MPTCP by itself is not a complete fix. This release does not change host kernel or MPTCP settings.

See the [Go Security Team advisory](https://groups.google.com/g/golang-announce/c/PCoQcdxoScw/m/aD-MgXEvAAAJ) and [Ubuntu CVE tracker](https://ubuntu.com/security/CVE-2026-98432). These external notices may change independently of this repository.
