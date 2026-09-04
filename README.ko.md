# screpdb

screpdb는 스타크래프트 리플레이를 위한 고급 분석 리포팅 도구입니다.

[English](README.md) | [한국어](README.ko.md)

[![Release](https://img.shields.io/github/v/release/marianogappa/screpdb)](https://github.com/marianogappa/screpdb/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/marianogappa/screpdb)](go.mod)
[![Coverage](https://img.shields.io/badge/coverage-82%25-brightgreen)](scripts/coverage.sh)
[![Replay load throughput](https://img.shields.io/badge/replay%20load-34.6%20replays%2Fsec-brightgreen)](.github/workflows/bench-load.yml)

## 주요 기능
### 고수준 의미 기반 특징으로 리플레이 필터링 및 검색
<img width="1670" alt="게임 목록: 고수준 의미 기반 특징으로 리플레이를 필터링하고 찾기" src="docs/images/game-list.png" />

### 게임 요약과 클릭 한 번으로 게임 클라이언트에서 볼 리플레이 관전 준비
<img width="1660" alt="게임 요약: 게임별 개요와 게임 클라이언트에서 볼 리플레이 관전 준비" src="docs/images/game-summary.png" />

### 맵 오버레이를 갖춘 풍부한 게임 이벤트 브라우저
<img width="1582" alt="맵 오버레이를 갖춘 풍부한 게임 이벤트 브라우저" src="docs/images/game-events.png" />

###  빌드 오더 감지, 차트, 프로게이머 타이밍 비교
<img width="1657" height="860" alt="빌드 오더 감지와 프로게이머 타이밍 비교 차트" src="https://github.com/user-attachments/assets/b3d909fd-17c6-410c-9bc9-fcba1cbf2313" />

###  실력 지표 측정: 화면 전환 멀티태스킹, 유닛 생산 리듬, 첫 유닛 효율
<img width="1643" alt="실력 지표: 화면 전환 멀티태스킹, 유닛 생산 리듬, 첫 유닛 효율" src="docs/images/skill-proxies.png" />

###  프로게이머 리플레이용 별칭 목록 지원(내장, 편집 가능, 가져오기/내보내기 가능)과 로컬 사용자 플레이어 이름 자동 별칭 처리
<img width="1133" height="629" alt="프로게이머 별칭 목록과 자동 별칭 처리" src="https://github.com/user-attachments/assets/592e773a-5691-4841-9d0e-5c53d8f22db4" />

### 정밀한 빌드 오더 감지와 타이밍 비교를 위한 정교한 초반 명령 중복 제거
<img width="1665" height="877" alt="초반 명령 중복 제거" src="https://github.com/user-attachments/assets/fcf5c796-89a8-4536-8d41-2ab4d868676c" />

### 멀티플레이어 밀리 게임의 동맹 타임라인과 팀 몰아주기 감지
<img width="1557" height="872" alt="동맹 타임라인과 팀 몰아주기 감지" src="https://github.com/user-attachments/assets/ce38f46a-89c8-4a9a-b9f9-6489afd9c05b" />

### 한국어 UI: 대시보드가 시스템 언어(English / 한국어)를 따르며 푸터에 전환 스위치가 있습니다



## 설치

릴리스 노트는 [CHANGELOG.md](CHANGELOG.md)를 참고하세요.

> ⚠️ **보안:** **Windows**에서는 screpdb가 워커를 **낮은 무결성(Low integrity)** 수준으로 실행합니다. OS가 screpdb의 모든 쓰기를 단일 앱 데이터 폴더로 제한하므로, 리플레이/맵 파서가 침해되더라도 컴퓨터의 다른 위치에는 쓸 수 없습니다([영어 README의 Security / I/O model 참고](README.md#security--io-model)). **macOS와 Linux**에는 아직 OS 샌드박스가 없습니다. screpdb는 자체 I/O를 모두 프로세스 내 파사드를 통해 처리하지만(쓰기는 앱 데이터 디렉터리와 리플레이 폴더로 제한, 사용자가 직접 실행한 자체 업데이트 외에는 외부 네트워크 호출 없음), 이는 OS 경계가 아닌 최선의 보호 장치이므로 실행 전에 신중히 판단해 주세요.

<details>
<summary><strong>Windows</strong>: Scoop으로 설치 권장</summary>

**👉 권장: [Scoop](https://scoop.sh)으로 설치하세요.** **PowerShell**을 열고 아래 명령을 붙여 넣습니다:

```powershell
scoop install git   # required by 'scoop bucket add' (skip if you already have git)
scoop bucket add screpdb https://github.com/marianogappa/screpdb
scoop install screpdb
```

이제 끝입니다. **`screpdb-gui`**를 실행하면 앱이 브라우저에서 열리고, CLI는 `screpdb`로 실행합니다.

나중에 업그레이드할 때는 다음 명령만 실행하면 됩니다:

```powershell
scoop update screpdb
```

> 💡 **예전 버전이 보이거나, `install`이 파일을 찾지 못해 실패하나요?** 로컬 버킷 사본은
> git 클론이며 `scoop update`를 실행할 때만 갱신됩니다. 먼저 (패키지 이름 없이)
> `scoop update`를 실행해 최신 매니페스트를 받은 뒤 `scoop install screpdb` /
> `scoop update screpdb`를 실행하세요.

Scoop이 권장 경로인 이유는 브라우저 없이 다운로드하므로 Windows가 "확인되지 않은 개발자" / SmartScreen 경고를 표시하지 **않고**, 업그레이드가 명령 하나로 끝나기 때문입니다. Scoop이 아직 없다면 먼저 설치하세요([scoop.sh](https://scoop.sh)의 한 줄 설치):

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
irm get.scoop.sh | iex
```

<details>
<summary>직접 다운로드를 원하시나요? (SmartScreen 경고가 표시됩니다)</summary>

[릴리스 페이지](https://github.com/marianogappa/screpdb/releases)에서 **`screpdb-gui-windows-amd64.exe`**(GUI. CLI는 `screpdb-windows-amd64.exe`)를 내려받아 더블 클릭하세요.

바이너리는 **코드 서명이 되어 있지 않아** 첫 실행 시 Windows가 경고할 수 있습니다. 아래 경고는 바이너리가 악성이라는 뜻이 아닙니다:

- **SmartScreen "Windows의 PC 보호"**: **추가 정보 → 실행**을 클릭하세요.
- **Microsoft Defender나 타사 백신**이 바이너리를 차단하거나 조용히 격리할 수 있습니다. 파일을 읽고 네트워크 요청을 하는 서명되지 않은 Go 바이너리는 잘 알려진 오탐 패턴입니다. 다운로드 폴더에서 파일이 사라졌다면 Defender의 보호 기록에서 복원하거나 제외 항목에 추가하세요.
- AppLocker나 Windows Defender 애플리케이션 제어가 적용된 **회사 PC**에서는 실행이 완전히 차단될 수 있습니다. 코드 서명 없이는 우회 방법이 없습니다.

GUI 바이너리는 콘솔이 없는 창 앱이므로 SmartScreen 대화 상자를 닫으면 그냥 시작되지 않고 오류도 출력하지 않습니다. Scoop을 쓰면 이런 문제가 모두 없습니다. [소스에서 빌드](#소스에서-빌드하기)할 수도 있습니다.

> 💡 **앱 내 업데이트 버튼이 작동하길 원하시나요?** `.exe`를 관리자 권한 없이 쓸 수 있는 폴더에 두세요. 예를 들어 `%LOCALAPPDATA%\Programs\screpdb\`를 만들어 거기에 넣으면 됩니다. screpdb는 자신의 폴더가 사용자 쓰기 가능일 때만 자체 바이너리를 교체할 수 있으므로 `C:\Program Files\`(관리자 권한 필요)에서는 자체 업데이트가 되지 않습니다. 이 경우 앱은 다운로드 링크만 표시합니다.

</details>

Scoop 매니페스트는 [`bucket/screpdb.json`](bucket/screpdb.json)에 있으며 릴리스마다 자동으로 갱신됩니다.

</details>

<details>
<summary><strong>Linux</strong>: 한 줄 설치 스크립트 또는 Homebrew</summary>

**명령 하나로 설치**(알맞은 바이너리를 내려받아 릴리스의 서명된 `SHA256SUMS`와 대조한 뒤 PATH에 넣어 줍니다):

```bash
curl -fsSL https://raw.githubusercontent.com/marianogappa/screpdb/main/install.sh | sh
```

그다음 `screpdb`를 실행하세요. 업그레이드는 같은 명령을 다시 실행하거나 앱 내 **업데이트** 버튼을 사용하세요.

> 🔍 **읽지 않은 스크립트를 파이프로 실행하지 마세요.** [`install.sh`](install.sh)는 1분 안에 검토할 수 있도록 의도적으로 짧고 의존성이 없습니다. 사용 중인 OS/아키텍처용 바이너리를 내려받아 릴리스의 서명된 `SHA256SUMS`와 대조하고 `~/.local/bin`에 복사하는 일만 합니다. 먼저 읽어 본 뒤 로컬 사본을 실행하려면:
>
> ```bash
> curl -fsSL https://raw.githubusercontent.com/marianogappa/screpdb/main/install.sh -o screpdb-install.sh
> less screpdb-install.sh   # audit it
> sh screpdb-install.sh
> ```

**[Homebrew](https://brew.sh) / Linuxbrew**를 선호하시나요?

```bash
brew install marianogappa/screpdb/screpdb   # upgrade later: brew upgrade screpdb
```

또는 [릴리스 페이지](https://github.com/marianogappa/screpdb/releases)에서 아키텍처에 맞는 바이너리를 내려받아 실행 권한을 주고 `PATH`에 있는 폴더로 옮기세요. 앱 내 **업데이트** 버튼이 작동하도록 쓰기 가능한 폴더(Homebrew prefix 제외)에 두는 것이 좋습니다:

```bash
chmod +x screpdb-linux-amd64                              # or screpdb-linux-arm64
mkdir -p ~/.local/bin && mv screpdb-linux-amd64 ~/.local/bin/screpdb
```

`~/.local/bin`은 한 줄 설치 스크립트의 기본 위치이며, `PATH`에 있는 쓰기 가능한 폴더라면 어디든 됩니다. curl/brew로 받은 바이너리에는 격리 플래그가 붙지 않으므로 바로 실행됩니다.

> 💡 screpdb는 자신의 폴더가 사용자 쓰기 가능이고 패키지 관리자 소유가 아닐 때만 자체 업데이트합니다. `~/Downloads`나 Homebrew prefix에서 실행한 바이너리는 자동 업데이트되지 않으며, 대신 앱이 다운로드 명령을 표시합니다.

</details>

<details>
<summary><strong>macOS</strong>: Homebrew 또는 한 줄 설치 스크립트</summary>

**[Homebrew](https://brew.sh)로 설치:**

```bash
brew install marianogappa/screpdb/screpdb   # upgrade later: brew upgrade screpdb
```

또는 한 줄 설치 스크립트를 사용하세요(릴리스의 서명된 `SHA256SUMS`와 대조하고 `~/.local/bin`에 설치합니다):

```bash
curl -fsSL https://raw.githubusercontent.com/marianogappa/screpdb/main/install.sh | sh
```

`sh`로 파이프하는 것이 걱정되시나요? 위 Linux 절에 나온 것과 같은 [`install.sh`](install.sh)입니다. 먼저 읽어 본 뒤 로컬 사본을 실행하세요.

그다음 `screpdb`를 실행하세요. 두 방법 모두 **Gatekeeper의 "확인되지 않은 개발자" 차단이 없습니다.** `brew`와 `curl`은 차단을 유발하는 격리 속성을 붙이지 않으므로 바이너리가 바로 실행됩니다(공증 불필요).

<details>
<summary>직접 다운로드를 원하시나요? (이 방법은 Gatekeeper에 <em>걸립니다</em>)</summary>

[릴리스 페이지](https://github.com/marianogappa/screpdb/releases)에서 아키텍처에 맞는 바이너리를 내려받은 뒤 격리 플래그를 지우고 `PATH`에 있는 폴더로 옮기세요. 앱 내 **업데이트** 버튼이 작동하도록 쓰기 가능한 폴더(Homebrew prefix 제외)를 사용하세요:

```bash
chmod +x screpdb-darwin-arm64                          # or screpdb-darwin-amd64
xattr -d com.apple.quarantine screpdb-darwin-arm64     # clear the browser-download quarantine
mkdir -p ~/.local/bin && mv screpdb-darwin-arm64 ~/.local/bin/screpdb
```

(또는 바이너리를 우클릭 → **열기**로 한 번 승인해도 됩니다.) `~/.local/bin`은 한 줄 설치 스크립트의 기본 위치와 같습니다.

> 💡 screpdb는 자신의 폴더가 사용자 쓰기 가능이고 패키지 관리자 소유가 아닐 때만 자체 업데이트합니다. `~/Downloads`나 Homebrew prefix에서 바로 실행한 바이너리는 자동 업데이트되지 않으며, 대신 앱이 다운로드 명령을 표시합니다.

</details>

</details>

### 소스에서 빌드하기

Go 1.25.2 이상이 필요합니다. 내장 대시보드 UI 자산을 먼저 다시 빌드하도록 (`go build`만 실행하지 말고) `make build`를 사용하세요:

```bash
git clone https://github.com/marianogappa/screpdb.git
cd screpdb
make build
```

## 제거

**1. 바이너리 삭제**

| 설치 방법 | 명령 |
| --- | --- |
| Scoop (Windows) | `scoop uninstall screpdb` |
| Homebrew (macOS/Linux) | `brew uninstall screpdb` |
| 한 줄 설치 스크립트 / 수동 설치 | 직접 넣은 바이너리를 삭제하세요 (예: `~/.local/bin/screpdb`) |

**2. 데이터 폴더 삭제**(선택 사항. 나중에 다시 설치할 때 데이터를 유지하려면 건너뛰세요.)

```bash
# Windows (PowerShell)
Remove-Item -Recurse -Force "$env:LOCALAPPDATA\screpdb"

# macOS
rm -rf "$HOME/Library/Application Support/screpdb"

# Linux
rm -rf "${XDG_CONFIG_HOME:-$HOME/.config}/screpdb"
```

## 한국어 UI

브라우저(시스템) 언어가 한국어이면 대시보드가 자동으로 한국어로 표시됩니다. 오른쪽 아래 푸터의 English / 한국어 전환 스위치로 언제든 언어를 바꿀 수 있습니다.

번역 수정 제안은 [이슈](https://github.com/marianogappa/screpdb/issues/new/choose)로 환영합니다. 번역 텍스트는 `internal/dashboard/frontend/src/locales/ko/`에 있습니다.

## 개발자 기능

UI 없이 `screpdb ingest`로 리플레이를 SQLite 데이터베이스에 불러와 직접 쿼리할 수 있고, `screpdb mcp`로 MCP 클라이언트(Claude Desktop, Claude Code, Cursor 등)에서 게임, 플레이어, 종족전, 빌드 오더에 대해 자연어로 질문하면 읽기 전용 SQL로 답을 얻을 수 있습니다. `screpdb dashboard --headless`는 UI 없이 JSON API 서버만 실행하며, 모든 UI 기능이 OpenAPI 스키마와 함께 API로 제공됩니다. 명령별 옵션은 [영어 README의 Developer features 참고](README.md#developer-features).

## 사양: 수치가 계산되는 방식

"9풀", "스포닝 풀이 6초 늦음", "질럿 생산 25.2초" 같은 앱의 모든 골든 값(유닛 이름, 빌드 시간, 프로 타이밍, 비용, 테크 트리 규칙, 감지 임계값)은 [SPECIFICATION.md](SPECIFICATION.md)에 문서화되어 있습니다. 이 문서는 Go 소스에서 `go generate ./...`로 생성되고 테스트로 검증되므로 코드와 어긋날 수 없습니다. 릴리스 다운로드를 `SHA256SUMS`와 minisign 서명으로 검증하는 방법도 같은 절에 있습니다. [영어 README의 Specification 참고](README.md#specification--how-the-numbers-are-computed).

## 보안 / I/O 모델

screpdb는 모든 디스크와 네트워크 접근을 `internal/iofacade` 등 파사드로만 처리해 공격 표면을 최소화합니다. 쓰기는 OS별 단일 앱 데이터 디렉터리와 설정된 리플레이 폴더로 제한되고, 대시보드 서버는 `localhost`에만 바인딩되며, 외부 호출은 사용자가 직접 실행하는 자체 업데이트(minisign 서명 검증)와 배틀넷 리플레이 다운로드에 한정됩니다. Windows에서는 낮은 무결성 워커가 실제 OS 쓰기 경계를 추가하고, `TestNoDirectIOOutsideFacades` 테스트가 파사드를 우회하는 코드를 CI에서 차단합니다. 자세한 내용과 I/O 안전 감사 기록은 [영어 README의 Security / I/O model 참고](README.md#security--io-model).

## 라이선스, 기여 및 감사의 말

이 프로젝트는 MIT 라이선스로 배포됩니다([LICENSE](LICENSE) 참고). 보안상의 이유로 코드 PR은 받지 않지만 [이슈](https://github.com/marianogappa/screpdb/issues) 제출과 코드 외 기여는 언제나 환영합니다. 리플레이 파싱은 [András Belicza](https://github.com/icza)의 [github.com/icza/screp](https://github.com/icza/screp) 라이브러리에 기반합니다. [영어 README의 License, Contributing & Acknowledgements 참고](README.md#license-contributing--acknowledgements).
