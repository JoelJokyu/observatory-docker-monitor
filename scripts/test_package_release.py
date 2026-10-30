import tempfile
import unittest
from pathlib import Path
import zipfile
from package_release import package

class PackagingTests(unittest.TestCase):
    def test_bundle_contains_only_allowlisted_files_and_no_password(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = package('0.1.0','ghcr.io/example/observatory:0.1.0',tmp)
            first = path.read_bytes()
            with zipfile.ZipFile(path) as z:
                names = z.namelist()
                self.assertIn('observatory-0.1.0/.env.example',names)
                self.assertNotIn('observatory-0.1.0/.env',names)
                self.assertFalse(any('/work/' in n or '/node_modules/' in n for n in names))
                env = z.read('observatory-0.1.0/.env.example').decode()
                self.assertIn('POSTGRES_PASSWORD=\n',env)
                self.assertNotIn('__OBSERVATORY_IMAGE__',env)
                self.assertIn('ghcr.io/example/observatory:0.1.0',env)
                self.assertIn('LICENSES/manifest.json','\n'.join(names))
            package('0.1.0','ghcr.io/example/observatory:0.1.0',tmp)
            self.assertEqual(first,path.read_bytes())
            self.assertTrue((Path(tmp)/'SHA256SUMS').exists())
    def test_rejects_floating_or_mismatched_tags_and_unsafe_versions(self):
        with tempfile.TemporaryDirectory() as tmp:
            for version,image in [('0.1.0','repo:latest'),('0.1.0','repo:0.2.0'),('../bad','repo:0.1.0'),('01.1.0','repo:01.1.0'),('0.1.0','repo\n:0.1.0')]:
                with self.assertRaises(ValueError): package(version,image,tmp)
    def test_accepts_immutable_digest(self):
        with tempfile.TemporaryDirectory() as tmp:
            self.assertTrue(package('0.1.0','ghcr.io/example/observatory@sha256:'+'a'*64,tmp).exists())

if __name__ == '__main__': unittest.main()
