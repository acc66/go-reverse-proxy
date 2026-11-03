import importlib.util
import json
import pathlib
import subprocess
import tempfile
import unittest

spec=importlib.util.spec_from_file_location("sites",pathlib.Path(__file__).with_name("fango-sites.py"))
module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)

class DomainTests(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.calls=[];self.fail=None;self.hosts=[]
        def run(args):
            self.calls.append(args)
            if self.fail and self.fail(args):
                raise subprocess.CalledProcessError(1,args,stderr="test failure")
            if args[0:2]==["caddy","adapt"]:
                return json.dumps({"match":[{"host":self.hosts}]})
            return ""
        self.s=module.Sites(self.temp.name,run)
        self.s.write(self.s.main,":80 {\n respond ok\n}\n",0o644)
        self.s.write(self.s.base_env,"UPSTREAM=https://api10.seox9.com/\nLISTEN_ADDR=127.0.0.1:8080\nCACHE_BYPASS_PATHS=/api,/user\n")

    def test_shared_and_duplicate(self):
        self.s.add("one.example.com");self.s.add("two.example.com")
        self.assertEqual(self.s.load()["one.example.com"]["port"],8080)
        before=len(self.calls)
        self.s.add("one.example.com")
        self.assertEqual(len(self.calls),before)
        self.assertFalse(any("restart" in cmd for cmd in self.calls))

    def test_isolated_update_remove(self):
        self.s.add("a.example.com","https://a.example.org/")
        self.s.add("b.example.com","https://b.example.org/")
        state=self.s.load()
        self.assertNotEqual(state["a.example.com"]["port"],state["b.example.com"]["port"])
        self.s.add("a.example.com","https://new.example.org/")
        self.assertEqual(self.s.load()["a.example.com"]["port"],state["a.example.com"]["port"])
        self.s.remove("a.example.com")
        self.assertNotIn("a.example.com",self.s.load())
        self.assertIn("b.example.com",self.s.load())

    def test_rollback_validate_failure(self):
        original=self.s.main.read_text()
        self.fail=lambda args:args[:2]==["caddy","validate"]
        with self.assertRaises(subprocess.CalledProcessError):
            self.s.add("bad.example.com","https://a.example.org/")
        self.assertEqual(self.s.main.read_text(),original)
        self.assertFalse(self.s.state.exists())
        self.assertFalse(self.s.files("bad.example.com")[0].exists())

    def test_failed_update_preserves_old(self):
        self.s.add("a.example.com","https://a.example.org/")
        before=self.s.load()
        oldenv=self.s.files("a.example.com")[1].read_text()
        self.fail=lambda args:args[:2]==["caddy","validate"]
        with self.assertRaises(subprocess.CalledProcessError):
            self.s.add("a.example.com","https://b.example.org/")
        self.assertEqual(self.s.load(),before)
        self.assertEqual(self.s.files("a.example.com")[1].read_text(),oldenv)

    def test_conflict_and_validation(self):
        self.hosts=["old.example.com","*.wild.example.com"]
        for domain in ["old.example.com","a.wild.example.com"]:
            with self.assertRaises(ValueError):self.s.add(domain)
        for domain in ["../bad","https://a.example.com","a.example.com {","*.example.com"]:
            with self.assertRaises(ValueError):self.s.add(domain)
        with self.assertRaises(ValueError):self.s.add("a.example.com","https://user:pass@example.com")
        with self.assertRaises(ValueError):self.s.remove("unmanaged.example.com")

    def test_private_settings_copied_and_shared_switch(self):
        self.s.add("a.example.com","https://a.example.org/")
        env=self.s.files("a.example.com")[1]
        self.assertIn("CACHE_BYPASS_PATHS=/api,/user",env.read_text())
        self.s.add("a.example.com")
        self.assertEqual(self.s.load()["a.example.com"]["port"],8080)
        self.assertFalse(env.exists())

    def test_remove_failure_restores_site(self):
        self.s.add("a.example.com","https://a.example.org/")
        original=self.s.load()
        self.fail=lambda args:args[:2]==["systemctl","reload"]
        with self.assertRaises(subprocess.CalledProcessError):
            self.s.remove("a.example.com")
        self.assertEqual(self.s.load(),original)
        self.assertTrue(self.s.files("a.example.com")[0].exists())

    def test_manual_edit_not_overwritten(self):
        self.s.add("a.example.com")
        route=self.s.files("a.example.com")[0]
        route.write_text("manually edited")
        with self.assertRaises(ValueError):self.s.add("a.example.com","https://other.example.org/")
        with self.assertRaises(ValueError):self.s.remove("a.example.com")
        self.assertEqual(route.read_text(),"manually edited")

    def test_failed_start_cleans_new_instance(self):
        self.fail=lambda args:args[:2]==["systemctl","is-active"]
        with self.assertRaises(subprocess.CalledProcessError):
            self.s.add("a.example.com","https://a.example.org/")
        route,env,unit_file,unit=self.s.files("a.example.com")
        self.assertFalse(env.exists())
        self.assertFalse(unit_file.exists())
        self.assertIn(["systemctl","disable","--now",unit],self.calls)

if __name__=="__main__":unittest.main()
