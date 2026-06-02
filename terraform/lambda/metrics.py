import os

import boto3

dynamodb = boto3.client("dynamodb")
cloudwatch = boto3.client("cloudwatch")

NAMESPACE = "plusplus"

# Metric name -> table name. Workspaces table is one item per team_id, so its
# count is unique workspaces. Karma table is keyed (team_id, user_id), so its
# count is user-workspace rows -- the closest thing to "unique users" the data
# supports (a Slack user_id is per-workspace and cannot be deduped to a human).
TABLES = {
    "Workspaces": os.environ["WORKSPACES_TABLE"],
    "Users": os.environ["KARMA_TABLE"],
    "ChannelSettings": os.environ["SETTINGS_TABLE"],
}


def count_items(table):
    total = 0
    kwargs = {"TableName": table, "Select": "COUNT"}
    while True:
        resp = dynamodb.scan(**kwargs)
        total += resp["Count"]
        start_key = resp.get("LastEvaluatedKey")
        if not start_key:
            return total
        kwargs["ExclusiveStartKey"] = start_key


def handler(event, context):
    counts = {name: count_items(table) for name, table in TABLES.items()}
    cloudwatch.put_metric_data(
        Namespace=NAMESPACE,
        MetricData=[
            {"MetricName": name, "Value": value, "Unit": "Count"}
            for name, value in counts.items()
        ],
    )
    return counts
