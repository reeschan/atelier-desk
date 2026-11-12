# Go + Wails で作り、Atelier のルールは Atelier の Python CLI にだけ置く

Atelier Desk は Go + Wails (画面は React + TypeScript) で作る。部品・フロー・モデル選択の検証や判定は Go で書き直さず、Atelier 同梱の Python CLI を子プロセスで呼んで、その JSON 出力 (`schema` の版つき) を表示する。足りない出力は Atelier 側に PR で足す。Atelier のルールを 2 つの言語で持つと、Atelier の版が上がるたびに食い違いが出るため。Go を選んだのは、アプリの裏側の仕事が子プロセスの管理・並列実行・ファイル監視・単一 exe の配布に集まり、そこに Go が向いているため。

## Considered Options

- **Tauri (Rust)**: 同じく軽いが、Rust を新しく持ち込む理由がない。
- **Electron**: 部品は最も多いが重く、裏側の仕事に対して利点がない。
- **Python + pywebview**: Atelier の関数を直接呼べるが、配布と長い処理の並列管理が弱い。直接呼べることは、ルールを CLI 越しに 1 か所に置く方針とむしろ相反する。

## Consequences

- アプリの機能は Atelier の CLI の出力に縛られる。画面に要る情報が出ていなければ、まず Atelier に PR を出す。
- home の JSON (`agents.json`・`routing.json` など) だけは、形式が決まっているので直接読み書きする。ただし書く前に Atelier の検証 CLI を通す。
