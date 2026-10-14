#!/usr/bin/env python3
"""Exercise release commands offline; never call the real GitHub CLI."""
import json
import os
import pathlib
import subprocess
import tempfile
import unittest

WORKFLOWS = pathlib.Path(__file__).resolve().parents[2] / ".github/workflows"


def read_workflow(path):
    # These checks run on macOS, using its bundled Ruby/Psych YAML parser.
    result = subprocess.check_output([
        "ruby", "-rjson", "-ryaml", "-e",
        "puts JSON.generate(YAML.safe_load(File.read(ARGV.fetch(0))))", str(path)
    ], text=True)
    return json.loads(result)


class DraftWorkflowTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.workflows = {p.name: read_workflow(p) for p in WORKFLOWS.glob("*.yml")}

    def release_script(self, job):
        steps = self.workflows["release.yml"]["jobs"][job]["steps"]
        commands = [step["run"] for step in steps if "gh release create" in step.get("run", "")]
        self.assertEqual(len(commands), 1, "expected one active draft-creation step")
        return commands[0]

    def invoke(self, job, tag, channel, exit_code=0):
        with tempfile.TemporaryDirectory(prefix="quosparr-draft-test-") as temporary:
            root = pathlib.Path(temporary)
            capture = root / "calls.jsonl"
            recorder = root / "gh"
            recorder.write_text(
                "#!/usr/bin/env python3\nimport json, os, sys\n"
                "with open(os.environ['GH_CAPTURE'], 'a') as log:\n"
                "    log.write(json.dumps(sys.argv[1:]) + '\\n')\n"
                "sys.exit(int(os.environ['GH_CAPTURE_EXIT']))\n")
            recorder.chmod(0o755)
            (root / "dist-v1").mkdir()
            (root / "dist-v1/payload.tar.gz").write_bytes(b"command-fixture")
            (root / "release-notes.md").write_text("Draft fixture\n")
            env = dict(os.environ, PATH=str(root) + os.pathsep + os.environ["PATH"],
                       GH_CAPTURE=str(capture), GH_CAPTURE_EXIT=str(exit_code),
                       GH_TOKEN="offline-command-recorder", GITHUB_REF_NAME=tag,
                       GITHUB_REPOSITORY="fixture/quosparr", GITHUB_SHA="0" * 40,
                       V1_UPDATE_CHANNEL=channel, BASH_ENV="/dev/null")
            result = subprocess.run(["bash", "--noprofile", "--norc", "-euo", "pipefail", "-c", self.release_script(job)],
                                    cwd=root, env=env, capture_output=True, text=True, timeout=10)
            calls = [json.loads(line) for line in capture.read_text().splitlines()]
            return result, calls

    def test_stable_and_beta_only_create_tagged_drafts(self):
        for job, tag, channel in (("v1-release", "v1.0.0", "stable"),
                                  ("v1-release", "beta-v1.0.0-beta.1", "beta")):
            with self.subTest(job=job, channel=channel):
                result, calls = self.invoke(job, tag, channel)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(len(calls), 1, "creating the draft is a single gh call")
                args = calls[0]
                self.assertEqual(args[:3], ["release", "create", tag])
                for flag in ("--draft", "--latest=false", "--verify-tag", "--notes-file"):
                    self.assertIn(flag, args)
                self.assertEqual("--prerelease" in args, channel == "beta")
                self.assertTrue(any(arg.endswith((".zip", ".tar.gz")) for arg in args), "draft lost its assets")

    def test_draft_creation_failure_stops_the_job(self):
        for job, channel in (("v1-release", "stable"), ("v1-release", "beta")):
            with self.subTest(job=job, channel=channel):
                result, calls = self.invoke(job, "fixture-tag", channel, exit_code=23)
                self.assertEqual(result.returncode, 23)
                self.assertEqual(len(calls), 1)

    def test_only_stable_tags_publish_as_latest(self):
        publish = []
        for name, workflow in self.workflows.items():
            for job_id, job in workflow["jobs"].items():
                for step in job.get("steps", []):
                    if "gh release edit" in step.get("run", ""):
                        publish.append((name, job_id, step))
        self.assertEqual(len(publish), 1, "expected one active publication step")
        name, job_id, step = publish[0]
        self.assertEqual((name, job_id), ("release.yml", "v1-release"))
        self.assertEqual(step["if"], "needs.v1-preflight.outputs.channel == 'stable'")
        steps = self.workflows[name]["jobs"][job_id]["steps"]
        self.assertGreater(steps.index(step), next(i for i, s in enumerate(steps) if "gh release create" in s.get("run", "")))
        self.assertEqual(step["run"], 'gh release edit "$GITHUB_REF_NAME" --repo "$GITHUB_REPOSITORY" --draft=false --latest=true')

    def test_beta_channel_and_distribution_steps_are_inactive(self):
        forbidden = ("gh release upload", "gh release create v1-beta", "generate-v1-homebrew.py")
        for workflow in self.workflows.values():
            for job in workflow["jobs"].values():
                for step in job.get("steps", []):
                    for command in forbidden:
                        self.assertNotIn(command, step.get("run", ""))

    def test_only_draft_jobs_have_repository_write_permission(self):
        for name, workflow in self.workflows.items():
            self.assertEqual(workflow["permissions"], {"contents": "read"})
            for job_id, job in workflow["jobs"].items():
                write = job.get("permissions", {}).get("contents") == "write"
                self.assertEqual(write, name == "release.yml" and job_id == "v1-release")


if __name__ == "__main__":
    unittest.main()
