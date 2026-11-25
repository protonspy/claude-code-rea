# Rea launcher — tasks

## 1 · Configuration

- [x] 1.1 (Unit) Define the provider table and resolve the provider and the model — R1.1, R1.2, R1.3, R1.5
- [x] 1.2 (TDD) Build the OpenCode configuration as ordered JSON with the permission rules — R1.4, R4.1, R5.1, R5.2, R5.3, R5.4
  _Depends 1.1_

## 2 · The provider key

- [x] 2.1 (TDD) Sign in to NanoGPT with OAuth PKCE on a loopback callback — R2.3, R2.4
- [x] 2.2 (Unit) Resolve the key from the variable, the store, sign-in or the prompt, and save or delete it — R2.1, R2.2, R2.5, R2.6, R2.7, R2.8, R2.9, R2.12
  _Depends 1.1, 2.1_

## 3 · The managed toolchain

- [x] 3.1 (Unit) Extract zip and tar.gz archives, refusing any entry that escapes the destination — R3.4
- [x] 3.2 (TDD) Download an archive, verify it against its pinned hash and publish it by rename — R3.2, R3.3, R3.4, R3.5
  _Depends 3.1_
- [x] 3.3 (Unit) Pin and install Node.js, REA and OpenCode for the current platform — R3.1, R3.6
  _Depends 3.2_

## 4 · REA

- [x] 4.1 (Unit) Write the reverse-engineering instructions embedded as AGENTS.md — R4.7
- [x] 4.2 (Unit) Forward the engine environment and install REA's skills and the AGENTS.md — R4.2, R4.3, R4.7
  _Depends 3.3, 4.1_
- [x] 4.3 (Unit) Check REA's Ghidra engine with its doctor and save the paths setup is given — R4.4, R4.5, R4.6
  _Depends 3.3_

## 5 · Commands

- [ ] 5.1 (Unit) Run OpenCode with the configuration and the key, and return its exit code — R2.10, R2.11, R6.1, R6.3, R6.4
  _Depends 1.2, 2.2, 3.3_
- [ ] 5.2 (Unit) Wire launch, setup, logout, version and print-config — R6.1, R6.2, R6.5, R7.1, R7.2
  _Depends 4.2, 4.3, 5.1_

## 6 · CI and release

- [ ] 6.1 (Unit) Add the CI workflow that builds, vets and tests on Linux and Windows — R8.1
  _Depends 5.2_
- [ ] 6.2 (Unit) Add the release workflow with SHA256SUMS and build provenance — R8.2
  _Depends 6.1_
