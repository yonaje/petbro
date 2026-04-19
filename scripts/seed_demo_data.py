#!/usr/bin/env python3

import json
import sys
import time
import urllib.error
import urllib.request


AUTH_BASE = "http://127.0.0.1:8080"
USER_BASE = "http://127.0.0.1:8081"
FRIEND_BASE = "http://127.0.0.1:8082"
PASSWORD = "Petbro123!"


def request_json(method, url, payload=None, headers=None, expected=(200, 201)):
    body = None
    request_headers = {"Content-Type": "application/json"}
    if headers:
        request_headers.update(headers)
    if payload is not None:
        body = json.dumps(payload).encode("utf-8")

    request = urllib.request.Request(url, data=body, headers=request_headers, method=method)
    try:
        with urllib.request.urlopen(request, timeout=10) as response:
            response_body = response.read().decode("utf-8")
            if response.status not in expected:
                raise RuntimeError(f"{method} {url} returned unexpected status {response.status}: {response_body}")
            if not response_body:
                return response.status, {}
            return response.status, json.loads(response_body)
    except urllib.error.HTTPError as error:
        error_body = error.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"{method} {url} failed with status {error.code}: {error_body}") from error
    except urllib.error.URLError as error:
        raise RuntimeError(f"{method} {url} failed: {error}") from error


def auth_headers(token):
    return {"Authorization": f"Bearer {token}"}


def register_user(index, prefix):
    payload = {
        "email": f"{prefix}_user{index}@example.com",
        "password": PASSWORD,
        "name": f"Demo User {index}",
        "description": f"Generated demo account #{index}",
        "avatar": f"https://example.com/avatar/{index}.png",
    }
    request_json("POST", f"{AUTH_BASE}/register", payload=payload, expected=(201,))
    _, login_body = request_json(
        "POST",
        f"{AUTH_BASE}/login",
        payload={"email": payload["email"], "password": PASSWORD},
        expected=(200,),
    )
    token = login_body["access_token"]
    _, me_body = request_json("GET", f"{AUTH_BASE}/me", headers=auth_headers(token), expected=(200,))
    return {
        "id": int(me_body["user_id"]),
        "name": payload["name"],
        "email": payload["email"],
        "token": token,
    }


def send_request(from_user, to_user_id):
    request_json(
        "POST",
        f"{FRIEND_BASE}/friend-request",
        payload={"toUserId": to_user_id},
        headers=auth_headers(from_user["token"]),
        expected=(201,),
    )


def accept_request(to_user, from_user_id):
    request_json(
        "POST",
        f"{FRIEND_BASE}/friend-request/accept",
        payload={"fromUserId": from_user_id},
        headers=auth_headers(to_user["token"]),
        expected=(200,),
    )


def get_friends(user):
    _, body = request_json(
        "GET",
        f"{FRIEND_BASE}/friends",
        headers=auth_headers(user["token"]),
        expected=(200,),
    )
    return body


def get_incoming_requests(user):
    _, body = request_json(
        "GET",
        f"{FRIEND_BASE}/friend-requests/incoming",
        headers=auth_headers(user["token"]),
        expected=(200,),
    )
    return body


def get_recommendations(user, limit=5):
    _, body = request_json(
        "GET",
        f"{FRIEND_BASE}/friend-recommendations?limit={limit}",
        headers=auth_headers(user["token"]),
        expected=(200,),
    )
    return body


def list_users(user):
    _, body = request_json(
        "GET",
        f"{USER_BASE}/users?page=1&limit=20&sort=id&order=asc",
        headers=auth_headers(user["token"]),
        expected=(200,),
    )
    return body


def main():
    prefix = f"seed{int(time.time())}"
    users = []

    for index in range(1, 11):
        user = register_user(index, prefix)
        users.append(user)
        print(f"created user {user['id']}: {user['email']}")

    for user in users:
        list_users(user)

    accepted_pairs = []
    pending_pairs = []

    accepted_edges = [
        (0, 1),
        (0, 2),
        (0, 3),
        (1, 2),
        (1, 4),
        (2, 5),
        (3, 4),
        (3, 6),
        (4, 5),
        (4, 7),
        (5, 8),
        (6, 7),
        (7, 8),
        (8, 9),
    ]

    pending_edges = [
        (2, 9),
        (6, 9),
        (1, 8),
    ]

    for from_index, to_index in accepted_edges:
        from_user = users[from_index]
        to_user = users[to_index]
        send_request(from_user, to_user["id"])
        accept_request(to_user, from_user["id"])
        accepted_pairs.append((from_user["id"], to_user["id"]))
        print(f"accepted friendship {from_user['id']} -> {to_user['id']}")

    for from_index, to_index in pending_edges:
        from_user = users[from_index]
        to_user = users[to_index]
        send_request(from_user, to_user["id"])
        pending_pairs.append((from_user["id"], to_user["id"]))
        print(f"left pending request {from_user['id']} -> {to_user['id']}")

    for user in users:
        friends = get_friends(user)
        incoming = get_incoming_requests(user)
        recommendations = get_recommendations(user, limit=5)
        print(
            f"user {user['id']} summary: "
            f"friends={len(friends)} incoming={len(incoming)} recommendations={len(recommendations)}"
        )

    summary = {
        "prefix": prefix,
        "user_ids": [user["id"] for user in users],
        "accepted_friendships": accepted_pairs,
        "pending_requests": pending_pairs,
    }
    print(json.dumps(summary, ensure_ascii=True))


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
