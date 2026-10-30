"""Collect license/notice text for Go and bundled frontend runtime dependencies."""
import json
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[1]

def json_stream(text):
    decoder = json.JSONDecoder()
    while text.strip():
        text = text.lstrip()
        item, offset = decoder.raw_decode(text)
        yield item
        text = text[offset:]

def generate():
    raw = subprocess.check_output(['go', 'list', '-deps', '-json', './cmd/observatory'], cwd=ROOT, text=True, encoding='utf-8')
    modules = {}
    for item in json_stream(raw):
        m = item.get('Module', {})
        if m and not m.get('Main'):
            if m.get('Replace'):
                raise ValueError('Review replacement modules before releasing')
            modules[m['Path']] = m
    out = ROOT / 'LICENSES'
    out.mkdir(exist_ok=True)
    entries = []
    def collect(name, version, folder):
        files = sorted(p for p in folder.iterdir() if p.is_file() and p.name.upper().startswith(('LICENSE', 'LICENCE', 'COPYING', 'NOTICE')))
        if not files:
            raise ValueError('No license text found for ' + name)
        paths = []
        for source in files:
            filename = (name + '@' + version + '_' + source.name).replace('/', '_').replace(':', '_').replace(' ', '_') + '.txt'
            (out / filename).write_bytes(source.read_bytes())
            paths.append('LICENSES/' + filename)
        entries.append({'name': name, 'version': version, 'files': paths})
    for name, m in sorted(modules.items()):
        collect('go/' + name, m['Version'], Path(m['Dir']))
    goroot = subprocess.check_output(['go', 'env', 'GOROOT'], text=True).strip()
    goversion = subprocess.check_output(['go', 'env', 'GOVERSION'], text=True).strip()
    collect('Go standard library', goversion, Path(goroot))
    lock = json.loads((ROOT / 'frontend/package-lock.json').read_text(encoding='utf-8-sig'))
    for name, package in sorted(lock['packages'].items()):
        if name and not package.get('dev') and not package.get('optional'):
            collect('npm/' + name.removeprefix('node_modules/'), package['version'], ROOT / 'frontend' / name)
    manifest = {'dependencies': entries}
    (out / 'manifest.json').write_text(json.dumps(manifest, indent=2) + '\n', encoding='utf-8')
    notice = '# Third-party notices\n\nObservatory is MIT licensed. Dependencies keep their own licenses and notices.\n\n'
    for e in entries:
        notice += f"- {e['name']} {e['version']}: " + ', '.join(f'[{Path(p).name}]({p})' for p in e['files']) + '\n'
    notice += '\nThis inventory covers Go modules used by the application, the Go standard library used for this build, and non-optional frontend runtime packages from the lockfile. Regenerate after dependency/toolchain changes. Build tools and separately downloaded PostgreSQL/base images have their own licenses and notices; consult those upstream distributions. This inventory is not a claim that all dependency terms are MIT.\n'
    (ROOT / 'THIRD_PARTY_NOTICES.md').write_text(notice, encoding='utf-8')
    print(f'Collected notices for {len(entries)} dependencies')

if __name__ == '__main__':
    generate()
