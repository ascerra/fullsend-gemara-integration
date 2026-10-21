# SPDX-License-Identifier: Apache-2.0
import copy
from datetime import datetime, timezone
import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location(
    "collector", Path(__file__).with_name("collect_mlflow_judge.py")
)
collector = importlib.util.module_from_spec(spec)
spec.loader.exec_module(collector)


class JudgeCollectorTests(unittest.TestCase):
    def setUp(self):
        self.now = datetime(2026, 10, 20, tzinfo=timezone.utc)
        self.binding = dict(
            schema_version="judge-binding/v1",
            experiment_id="1",
            scorer="review_correctly_approved",
            source_id="synthetic.judge",
            lookback_days=365,
        )
        self.row = dict(
            trace_id="synthetic-trace-1",
            trace_location={"mlflow_experiment": {"experiment_id": "1"}},
            request_time="2026-10-19T00:00:00Z",
            state="OK",
            assessments=[
                dict(
                    assessment_id="synthetic-assessment-1",
                    assessment_name="review_correctly_approved",
                    source={"source_type": "LLM_JUDGE", "source_id": "synthetic.judge"},
                    valid=True,
                    feedback={"value": 1.0},
                    last_update_time="2026-10-19T01:00:00Z",
                    rationale="SYNTHETIC_PRIVATE_PROSE",
                )
            ],
        )

    def collect(self, pages):
        iterator = iter(pages)
        return collector.collect(self.binding, self.now, lambda _: next(iterator))

    def test_complete_paginated_source_filtered_aggregate_without_prose(self):
        second = copy.deepcopy(self.row)
        second["trace_id"] = "synthetic-trace-2"
        second["assessments"][0]["feedback"]["value"] = 0
        result = self.collect(
            [{"traces": [self.row], "next_page_token": "page2"}, {"traces": [second]}]
        )
        self.assertEqual(
            (result["passed"], result["total"], result["invalid"], result["complete"]),
            (1, 2, 0, True),
        )
        self.assertNotIn("SYNTHETIC_PRIVATE_PROSE", str(result))
        self.assertNotIn("synthetic-trace-1", str(result))
        self.assertEqual(result["authorization"], "none")
        self.assertFalse(result["mutation_permitted"])

    def test_invalid_or_duplicate_matching_scores_cannot_inflate_rate(self):
        for value in [True, "1", 0.5, float("nan"), -1]:
            with self.subTest(value=value):
                row = copy.deepcopy(self.row)
                row["assessments"][0]["feedback"]["value"] = value
                result = self.collect([{"traces": [row]}])
                self.assertEqual((result["total"], result["invalid"]), (0, 1))
        self.row["assessments"] *= 2
        result = self.collect([{"traces": [self.row]}])
        self.assertEqual((result["total"], result["invalid"]), (0, 1))

    def test_other_sources_excluded_but_bound_invalidated_scores_hold(self):
        row = copy.deepcopy(self.row)
        row["assessments"][0]["source"]["source_id"] = "other"
        r = self.collect([{"traces": [row]}])
        self.assertEqual(
            (r["total"], r["excluded_other_sources"], r["invalid"]), (0, 1, 0)
        )
        for change in [
            lambda a: a.update(valid=False),
            lambda a: a.pop("valid"),
            lambda a: a["source"].update(source_type="CODE"),
            lambda a: a["feedback"].update(error={"code": "synthetic_error"}),
        ]:
            row = copy.deepcopy(self.row)
            change(row["assessments"][0])
            r = self.collect([{"traces": [row]}])
            self.assertEqual(
                (r["total"], r["excluded_other_sources"], r["invalid"]), (0, 0, 1)
            )

    def test_two_scans_must_match_and_time_bounds_are_aligned(self):
        now = self.now.replace(microsecond=123456)
        bodies = []

        def fetch(body):
            bodies.append(copy.deepcopy(body))
            return {"traces": [copy.deepcopy(self.row)]}

        result = collector.collect_stable(self.binding, now, fetch)
        self.assertEqual(len(bodies), 2)
        self.assertEqual(result["window_end"], "2026-10-20T00:00:00Z")
        self.assertIn(
            "trace.timestamp_ms < " + str(int(self.now.timestamp() * 1000)),
            bodies[0]["filter"],
        )
        calls = 0

        def changed(body):
            nonlocal calls
            calls += 1
            row = copy.deepcopy(self.row)
            row["assessments"][0]["feedback"]["value"] = calls - 1
            return {"traces": [row]}

        with self.assertRaisesRegex(ValueError, "judge_population_changed"):
            collector.collect_stable(self.binding, now, changed)

    def test_duplicate_trace_pagination_cycle_wrong_experiment_and_window_reject(self):
        with self.assertRaisesRegex(ValueError, "duplicate_trace"):
            self.collect(
                [
                    {"traces": [self.row], "next_page_token": "page2"},
                    {"traces": [self.row]},
                ]
            )
        with self.assertRaisesRegex(ValueError, "pagination_cycle"):
            self.collect([{"traces": [], "next_page_token": "page2"}] * 2)
        row = copy.deepcopy(self.row)
        row["trace_location"]["mlflow_experiment"]["experiment_id"] = "2"
        with self.assertRaisesRegex(ValueError, "wrong_experiment"):
            self.collect([{"traces": [row]}])
        self.row["request_time"] = "2020-01-01T00:00:00Z"
        with self.assertRaisesRegex(ValueError, "trace_outside_window"):
            self.collect([{"traces": [self.row]}])

    def test_malformed_later_page_cannot_look_like_complete_population(self):
        for bad in [
            {"error_code": "synthetic_failure"},
            {"traces": None},
            {"traces": {}},
            {"traces": [], "next_page_token": 0},
        ]:
            with self.subTest(page=bad), self.assertRaisesRegex(
                ValueError, "invalid_search_page|invalid_page_token"
            ):
                self.collect([{"traces": [self.row], "next_page_token": "second"}, bad])

    def test_page_limit_is_incomplete_and_not_a_passing_subset(self):
        count = 0

        def fetch(_):
            nonlocal count
            count += 1
            return {"traces": [], "next_page_token": str(count)}

        result = collector.collect(self.binding, self.now, fetch)
        self.assertFalse(result["complete"])
        self.assertEqual(count, 50)


if __name__ == "__main__":
    unittest.main()
