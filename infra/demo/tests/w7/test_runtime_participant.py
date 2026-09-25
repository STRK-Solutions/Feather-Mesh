"""Offline checks for the reviewed participant runner transition."""
from pathlib import Path
import unittest

import yaml

ROOT = Path(__file__).resolve().parents[4]


class ParticipantRuntimeTests(unittest.TestCase):
    def test_runner_limits_apply_live_without_restarting_w1(self):
        play = yaml.safe_load((ROOT / 'infra/demo/ansible/runtime-participant.yml').read_text())[0]
        self.assertEqual((play['vars']['feam_runner_memory_max'],
                          play['vars']['feam_runner_cpu_quota'],
                          play['vars']['feam_runner_tasks_max']), ('6G', '600%', 2048))
        tasks = yaml.safe_load((ROOT / 'infra/demo/ansible/roles/runtime/tasks/main.yml').read_text())
        by_name = {task['name']: task for task in tasks}
        live = by_name['Apply changed runner slice limits live without restarting the manager']
        self.assertIn('--runtime', live['ansible.builtin.command']['argv'])
        self.assertEqual(live['when'], 'runtime_slice_unit.changed')
        self.assertFalse(any(task.get('become_user') == 'feam-runner' for task in tasks))
        self.assertFalse(any(task.get('ansible.builtin.systemd_service', {}).get('state') == 'restarted'
                             for task in tasks))
        for name in ('Inspect dedicated user daemon enablement',
                     'Enable the dedicated user daemon without restarting it',
                     'Inspect dedicated user daemon activity',
                     'Start the dedicated user daemon only when inactive',
                     'Verify rootless systemd cgroup support'):
            self.assertEqual(by_name[name]['ansible.builtin.command']['argv'][0], '/usr/sbin/runuser')
        self.assertEqual(by_name['Start the dedicated user daemon only when inactive']['when'],
                         "runtime_docker_activity.stdout == 'inactive'")


if __name__ == '__main__':
    unittest.main()
