"""Create an allowlisted end-user ZIP; never copy a checkout or local .env."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import zipfile

ROOT = Path(__file__).resolve().parents[1]
VERSION = re.compile(r'(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\Z')

def package(version, image, destination):
    if not VERSION.fullmatch(version):
        raise ValueError('Use a stable version such as 0.1.0 (without v)')
    if not re.fullmatch(r'(?:[a-z0-9]+(?:[._-][a-z0-9]+)*(?::[0-9]+)?/)*[a-z0-9]+(?:[._-][a-z0-9]+)*(?::[a-zA-Z0-9_.-]+|@sha256:[0-9a-f]{64})', image):
        raise ValueError('Use an explicit image tag or sha256 digest')
    if '@sha256:' not in image and image.rsplit(':', 1)[-1] != version:
        raise ValueError('Image tag must match the release version; floating tags are not allowed')
    files = {'compose.yaml': ROOT/'packaging/compose.yaml', 'INSTALL.md': ROOT/'packaging/INSTALL.md', 'LICENSE': ROOT/'LICENSE', 'THIRD_PARTY_NOTICES.md': ROOT/'THIRD_PARTY_NOTICES.md'}
    manifest = json.loads((ROOT/'LICENSES/manifest.json').read_text(encoding='utf-8'))
    files['LICENSES/manifest.json'] = ROOT/'LICENSES/manifest.json'
    for entry in manifest['dependencies']:
        for name in entry['files']:
            p = Path(name)
            if p.is_absolute() or '..' in p.parts or p.parts[0] != 'LICENSES':
                raise ValueError('Unsafe notice manifest path')
            files[p.as_posix()] = ROOT/p
    data = {name: path.read_bytes() for name, path in files.items()}
    data['.env.example'] = (ROOT/'packaging/.env.example').read_text(encoding='utf-8-sig').replace('__OBSERVATORY_IMAGE__', image).encode()
    data['release.json'] = (json.dumps({'version': version, 'image': image}, indent=2)+'\n').encode()
    destination = Path(destination)
    destination.mkdir(parents=True, exist_ok=True)
    name = 'observatory-'+version
    archive = destination/(name+'.zip')
    with zipfile.ZipFile(archive, 'w', compression=zipfile.ZIP_DEFLATED) as z:
        for relative, content in sorted(data.items()):
            info = zipfile.ZipInfo(name+'/'+relative, date_time=(2026,1,1,0,0,0))
            info.compress_type = zipfile.ZIP_DEFLATED
            info.external_attr = 0o100644 << 16
            z.writestr(info, content)
    digest = hashlib.sha256(archive.read_bytes()).hexdigest()
    (destination/'SHA256SUMS').write_text(f'{digest}  {archive.name}\n',encoding='utf-8')
    return archive

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--version', required=True)
    parser.add_argument('--image', required=True)
    parser.add_argument('--output', default='dist')
    args = parser.parse_args()
    print(package(args.version, args.image, args.output))
