// Sample data for the NODES 2026 demo graph.
//
// Run this against an empty database (after running schema.cypher) to get
// a fully populated demo graph without needing the sync pipeline yet.
//
// The data is intentionally designed to support the closing demo query:
//
//   "What did the team decide about the schema validation issue?"
//
// The agent will call getIssueDiscussionContext(identifier: "NODES-1") and
// the graph will return the issue, two threads where it was discussed,
// the messages in each thread, and the people involved.
//
// Six people, one project, six issues, one Slack channel, four threads,
// twelve messages. Small enough to fit on screen during the demo, rich
// enough to show real cross-system reasoning.
//
// NOTE: every statement in this file is self-contained. Cypher does not
// share variables across ';'-separated statements, so any variable a
// statement needs is re-MATCHed (or re-MERGED) by its key at the start
// of that statement.
//
// NOTE: apostrophes inside message text use the Unicode right single
// quotation mark (’ U+2019) rather than the '' escape. The Neo4j 2026.x
// HTTP/acceptor path used for this demo rejects '' escapes inside string
// literals; the Unicode form is display-identical for the demo text.

// ----- Project and Channel -----

MERGE (p:Project {id: 'proj_nodes_demo'})
SET p.name = 'NODES 2026 Demo Project';

MERGE (c:Channel {id: 'C09ABCDEF12'})
SET c.name = 'nodes-demo-eng',
    c.purpose = 'Engineering discussion for the NODES demo';

// ----- People -----

MERGE (alex:Person {email: 'alex@example.com'})
ON CREATE SET alex.id = randomUUID()
SET alex.name = 'Alex Rivera',
    alex.linearId = 'user_alex_linear',
    alex.slackId = 'U09ALEXID01';

MERGE (sarah:Person {email: 'sarah@example.com'})
ON CREATE SET sarah.id = randomUUID()
SET sarah.name = 'Sarah Chen',
    sarah.linearId = 'user_sarah_linear',
    sarah.slackId = 'U09SARAHID2';

MERGE (marcus:Person {email: 'marcus@example.com'})
ON CREATE SET marcus.id = randomUUID()
SET marcus.name = 'Marcus Webb',
    marcus.linearId = 'user_marcus_linear',
    marcus.slackId = 'U09MARCUSID';

MERGE (priya:Person {email: 'priya@example.com'})
ON CREATE SET priya.id = randomUUID()
SET priya.name = 'Priya Patel',
    priya.linearId = 'user_priya_linear',
    priya.slackId = 'U09PRIYAID4';

MERGE (tom:Person {email: 'tom@example.com'})
ON CREATE SET tom.id = randomUUID()
SET tom.name = 'Tom Nakamura',
    tom.linearId = 'user_tom_linear',
    tom.slackId = 'U09TOMID005';

// One person who exists only in Slack (no Linear ID) — realistic for
// non-engineering members who participate in channels but don't have Linear seats
MERGE (jess:Person {email: 'jess@example.com'})
ON CREATE SET jess.id = randomUUID()
SET jess.name = 'Jess Morgan',
    jess.slackId = 'U09JESSID006';

// ----- Issues -----

// NODES-1: the centerpiece issue. The closing demo query targets this one.
MERGE (i1:Issue {id: 'issue_nodes_1'})
SET i1.identifier = 'NODES-1',
    i1.title = 'Federation gateway rejects valid GraphQL queries with schema validation error',
    i1.state = 'In Progress',
    i1.priority = 4,
    i1.linearUrl = 'https://linear.app/example/issue/NODES-1',
    i1.createdAt = datetime('2026-05-10T09:00:00Z'),
    i1.updatedAt = datetime('2026-05-14T16:30:00Z');

MERGE (i2:Issue {id: 'issue_nodes_2'})
SET i2.identifier = 'NODES-2',
    i2.title = 'Linear subgraph returns 401 intermittently under load',
    i2.state = 'Backlog',
    i2.priority = 3,
    i2.linearUrl = 'https://linear.app/example/issue/NODES-2',
    i2.createdAt = datetime('2026-05-11T11:20:00Z'),
    i2.updatedAt = datetime('2026-05-12T08:45:00Z');

MERGE (i3:Issue {id: 'issue_nodes_3'})
SET i3.identifier = 'NODES-3',
    i3.title = 'Add OAuth refresh logic to MCP gateway',
    i3.state = 'Triage',
    i3.priority = 2,
    i3.linearUrl = 'https://linear.app/example/issue/NODES-3',
    i3.createdAt = datetime('2026-05-12T14:15:00Z'),
    i3.updatedAt = datetime('2026-05-12T14:15:00Z');

MERGE (i4:Issue {id: 'issue_nodes_4'})
SET i4.identifier = 'NODES-4',
    i4.title = 'Document persisted operations workflow for the team',
    i4.state = 'Backlog',
    i4.priority = 1,
    i4.linearUrl = 'https://linear.app/example/issue/NODES-4',
    i4.createdAt = datetime('2026-05-12T16:00:00Z'),
    i4.updatedAt = datetime('2026-05-13T10:00:00Z');

MERGE (i5:Issue {id: 'issue_nodes_5'})
SET i5.identifier = 'NODES-5',
    i5.title = 'Race condition in plugin loader when router restarts under traffic',
    i5.state = 'Done',
    i5.priority = 3,
    i5.linearUrl = 'https://linear.app/example/issue/NODES-5',
    i5.createdAt = datetime('2026-05-08T13:00:00Z'),
    i5.updatedAt = datetime('2026-05-11T17:00:00Z');

MERGE (i6:Issue {id: 'issue_nodes_6'})
SET i6.identifier = 'NODES-6',
    i6.title = 'Investigate persisted operation cache invalidation strategy',
    i6.state = 'Backlog',
    i6.priority = 2,
    i6.linearUrl = 'https://linear.app/example/issue/NODES-6',
    i6.createdAt = datetime('2026-05-13T09:30:00Z'),
    i6.updatedAt = datetime('2026-05-13T09:30:00Z');

// Connect issues to the project
MATCH (i1:Issue {id: 'issue_nodes_1'})
MATCH (i2:Issue {id: 'issue_nodes_2'})
MATCH (i3:Issue {id: 'issue_nodes_3'})
MATCH (i4:Issue {id: 'issue_nodes_4'})
MATCH (i5:Issue {id: 'issue_nodes_5'})
MATCH (i6:Issue {id: 'issue_nodes_6'})
MATCH (p:Project {id: 'proj_nodes_demo'})
MERGE (p)-[:HAS_ISSUE]->(i1)
MERGE (p)-[:HAS_ISSUE]->(i2)
MERGE (p)-[:HAS_ISSUE]->(i3)
MERGE (p)-[:HAS_ISSUE]->(i4)
MERGE (p)-[:HAS_ISSUE]->(i5)
MERGE (p)-[:HAS_ISSUE]->(i6);

// Issue authorship and assignment
MATCH (i1:Issue {id: 'issue_nodes_1'})
MATCH (i2:Issue {id: 'issue_nodes_2'})
MATCH (i3:Issue {id: 'issue_nodes_3'})
MATCH (i4:Issue {id: 'issue_nodes_4'})
MATCH (i5:Issue {id: 'issue_nodes_5'})
MATCH (i6:Issue {id: 'issue_nodes_6'})
MATCH (alex:Person {email: 'alex@example.com'})
MATCH (sarah:Person {email: 'sarah@example.com'})
MATCH (marcus:Person {email: 'marcus@example.com'})
MATCH (priya:Person {email: 'priya@example.com'})
MATCH (tom:Person {email: 'tom@example.com'})

MERGE (sarah)-[:CREATED]->(i1)
MERGE (sarah)-[:ASSIGNED_TO]->(i1)

MERGE (marcus)-[:CREATED]->(i2)
MERGE (marcus)-[:ASSIGNED_TO]->(i2)

MERGE (priya)-[:CREATED]->(i3)

MERGE (tom)-[:CREATED]->(i4)
MERGE (priya)-[:ASSIGNED_TO]->(i4)

MERGE (alex)-[:CREATED]->(i5)
MERGE (marcus)-[:ASSIGNED_TO]->(i5)

MERGE (sarah)-[:CREATED]->(i6);

// ----- Slack threads and messages -----
//
// Slack timestamps are Unix epoch seconds with microsecond precision.
// Approximate equivalents (UTC):
//   1778400000.000000 ≈ 2026-05-10 08:00:00
//   1778572800.000000 ≈ 2026-05-12 08:00:00
//   1778659200.000000 ≈ 2026-05-13 08:00:00

// Thread 1: explicit reference to NODES-1
// Sarah raises the schema validation problem in #nodes-demo-eng
MATCH (c:Channel {id: 'C09ABCDEF12'})
MATCH (sarah:Person {email: 'sarah@example.com'})
MATCH (marcus:Person {email: 'marcus@example.com'})
MATCH (alex:Person {email: 'alex@example.com'})

MERGE (t1:Thread {channelId: 'C09ABCDEF12', ts: '1778486400.000100'})
ON CREATE SET t1.startedAt = datetime({epochSeconds: 1778486400}),
              t1.permalink = 'https://example.slack.com/archives/C09ABCDEF12/p1778486400000100'
SET t1.messageCount = 4
MERGE (c)-[:HOSTS_THREAD]->(t1)

// Parent message
MERGE (m1_1:Message {channelId: 'C09ABCDEF12', ts: '1778486400.000100'})
SET m1_1.text = 'Anyone else seeing the federation gateway reject otherwise-valid queries? Looks like NODES-1 — schema validation is being too strict on null handling.',
    m1_1.postedAt = datetime({epochSeconds: 1778486400}),
    m1_1.threadTs = '1778486400.000100',
    m1_1.permalink = 'https://example.slack.com/archives/C09ABCDEF12/p1778486400000100'
MERGE (t1)-[:HAS_MESSAGE]->(m1_1)
MERGE (sarah)-[:AUTHORED]->(m1_1)

// Reply 1
MERGE (m1_2:Message {channelId: 'C09ABCDEF12', ts: '1778487200.000200'})
SET m1_2.text = 'Yeah I just hit it too. Looks like the gateway treats nullable fields as required when the subgraph schema declares them optional. Repro is in NODES-1.',
    m1_2.postedAt = datetime({epochSeconds: 1778487200}),
    m1_2.threadTs = '1778486400.000100',
    m1_2.permalink = 'https://example.slack.com/archives/C09ABCDEF12/p1778487200000200'
MERGE (t1)-[:HAS_MESSAGE]->(m1_2)
MERGE (marcus)-[:AUTHORED]->(m1_2)

// Reply 2
MERGE (m1_3:Message {channelId: 'C09ABCDEF12', ts: '1778488800.000300'})
SET m1_3.text = 'I think the fix is in the composition step — we’re composing the supergraph without preserving the nullability annotations on a few of the Linear types. Let me dig in.',
    m1_3.postedAt = datetime({epochSeconds: 1778488800}),
    m1_3.threadTs = '1778486400.000100',
    m1_3.permalink = 'https://example.slack.com/archives/C09ABCDEF12/p1778488800000300'
MERGE (t1)-[:HAS_MESSAGE]->(m1_3)
MERGE (sarah)-[:AUTHORED]->(m1_3)

// Reply 3 — decision
MERGE (m1_4:Message {channelId: 'C09ABCDEF12', ts: '1778490000.000400'})
SET m1_4.text = 'Sarah confirmed — composition was dropping the nullability. Pushed a fix, will land tomorrow. Going with the explicit @shareable annotation approach rather than relying on defaults. Let’s use that pattern across all three subgraphs.',
    m1_4.postedAt = datetime({epochSeconds: 1778490000}),
    m1_4.threadTs = '1778486400.000100',
    m1_4.permalink = 'https://example.slack.com/archives/C09ABCDEF12/p1778490000000400'
MERGE (t1)-[:HAS_MESSAGE]->(m1_4)
MERGE (alex)-[:AUTHORED]->(m1_4);

// Thread 2: also mentions NODES-1, plus NODES-5 (cross-issue conversation)
MATCH (c:Channel {id: 'C09ABCDEF12'})
MATCH (tom:Person {email: 'tom@example.com'})
MATCH (sarah:Person {email: 'sarah@example.com'})
MATCH (marcus:Person {email: 'marcus@example.com'})

MERGE (t2:Thread {channelId: 'C09ABCDEF12', ts: '1778572800.000100'})
ON CREATE SET t2.startedAt = datetime({epochSeconds: 1778572800}),
              t2.permalink = 'https://example.slack.com/archives/C09ABCDEF12/p1778572800000100'
SET t2.messageCount = 3
MERGE (c)-[:HOSTS_THREAD]->(t2)

MERGE (m2_1:Message {channelId: 'C09ABCDEF12', ts: '1778572800.000100'})
SET m2_1.text = 'Pre-mortem question for the demo: if NODES-1 and NODES-5 are both fixed but a new plugin gets pushed, would the gateway re-validate? Or does the cached composition stick around?',
    m2_1.postedAt = datetime({epochSeconds: 1778572800}),
    m2_1.threadTs = '1778572800.000100',
    m2_1.permalink = 'https://example.slack.com/archives/C09ABCDEF12/p1778572800000100'
MERGE (t2)-[:HAS_MESSAGE]->(m2_1)
MERGE (tom)-[:AUTHORED]->(m2_1)

MERGE (m2_2:Message {channelId: 'C09ABCDEF12', ts: '1778573400.000200'})
SET m2_2.text = 'Good question. Composition cache is keyed on the schema hash, so a plugin push triggers re-composition. NODES-5 fix should mean restart is now safe under load.',
    m2_2.postedAt = datetime({epochSeconds: 1778573400}),
    m2_2.threadTs = '1778572800.000100',
    m2_2.permalink = 'https://example.slack.com/archives/C09ABCDEF12/p1778573400000200'
MERGE (t2)-[:HAS_MESSAGE]->(m2_2)
MERGE (sarah)-[:AUTHORED]->(m2_2)

MERGE (m2_3:Message {channelId: 'C09ABCDEF12', ts: '1778574000.000300'})
SET m2_3.text = 'Worth opening NODES-6 to investigate cache invalidation strategy more formally. The hash-keyed approach works for now but it’s not great for partial updates.',
    m2_3.postedAt = datetime({epochSeconds: 1778574000}),
    m2_3.threadTs = '1778572800.000100',
    m2_3.permalink = 'https://example.slack.com/archives/C09ABCDEF12/p1778574000000300'
MERGE (t2)-[:HAS_MESSAGE]->(m2_3)
MERGE (marcus)-[:AUTHORED]->(m2_3);

// Thread 3: about NODES-3 (OAuth refresh) — separate concern
MATCH (c:Channel {id: 'C09ABCDEF12'})
MATCH (priya:Person {email: 'priya@example.com'})
MATCH (marcus:Person {email: 'marcus@example.com'})

MERGE (t3:Thread {channelId: 'C09ABCDEF12', ts: '1778580000.000100'})
ON CREATE SET t3.startedAt = datetime({epochSeconds: 1778580000}),
              t3.permalink = 'https://example.slack.com/archives/C09ABCDEF12/p1778580000000100'
SET t3.messageCount = 2
MERGE (c)-[:HOSTS_THREAD]->(t3)

MERGE (m3_1:Message {channelId: 'C09ABCDEF12', ts: '1778580000.000100'})
SET m3_1.text = 'Token refresh for the MCP gateway — anyone want to take NODES-3? Should be a small one but it’s blocking the long-running session tests.',
    m3_1.postedAt = datetime({epochSeconds: 1778580000}),
    m3_1.threadTs = '1778580000.000100',
    m3_1.permalink = 'https://example.slack.com/archives/C09ABCDEF12/p1778580000000100'
MERGE (t3)-[:HAS_MESSAGE]->(m3_1)
MERGE (priya)-[:AUTHORED]->(m3_1)

MERGE (m3_2:Message {channelId: 'C09ABCDEF12', ts: '1778580600.000200'})
SET m3_2.text = 'I can pick it up next week if no one else does. Looks straightforward.',
    m3_2.postedAt = datetime({epochSeconds: 1778580600}),
    m3_2.threadTs = '1778580000.000100',
    m3_2.permalink = 'https://example.slack.com/archives/C09ABCDEF12/p1778580600000200'
MERGE (t3)-[:HAS_MESSAGE]->(m3_2)
MERGE (marcus)-[:AUTHORED]->(m3_2);

// Thread 4: no explicit issue mention — about "the schema problem"
// This thread exists so iteration 2's vector linking has something to find
// that the regex pass misses. For iteration 1, it stays orphaned.
MATCH (c:Channel {id: 'C09ABCDEF12'})
MATCH (jess:Person {email: 'jess@example.com'})
MATCH (alex:Person {email: 'alex@example.com'})

MERGE (t4:Thread {channelId: 'C09ABCDEF12', ts: '1778659200.000100'})
ON CREATE SET t4.startedAt = datetime({epochSeconds: 1778659200}),
              t4.permalink = 'https://example.slack.com/archives/C09ABCDEF12/p1778659200000100'
SET t4.messageCount = 3
MERGE (c)-[:HOSTS_THREAD]->(t4)

MERGE (m4_1:Message {channelId: 'C09ABCDEF12', ts: '1778659200.000100'})
SET m4_1.text = 'Quick one — has the schema problem from earlier this week been resolved? I’m about to onboard a new subgraph and want to know if the nullability issue is going to bite me.',
    m4_1.postedAt = datetime({epochSeconds: 1778659200}),
    m4_1.threadTs = '1778659200.000100',
    m4_1.permalink = 'https://example.slack.com/archives/C09ABCDEF12/p1778659200000100'
MERGE (t4)-[:HAS_MESSAGE]->(m4_1)
MERGE (jess)-[:AUTHORED]->(m4_1)

MERGE (m4_2:Message {channelId: 'C09ABCDEF12', ts: '1778659800.000200'})
SET m4_2.text = 'Yes — fix landed yesterday. You should be fine. Use the explicit @shareable pattern when you compose your subgraph schema.',
    m4_2.postedAt = datetime({epochSeconds: 1778659800}),
    m4_2.threadTs = '1778659200.000100',
    m4_2.permalink = 'https://example.slack.com/archives/C09ABCDEF12/p1778659800000200'
MERGE (t4)-[:HAS_MESSAGE]->(m4_2)
MERGE (alex)-[:AUTHORED]->(m4_2)

MERGE (m4_3:Message {channelId: 'C09ABCDEF12', ts: '1778660400.000300'})
SET m4_3.text = 'Perfect, thanks. Will follow that pattern.',
    m4_3.postedAt = datetime({epochSeconds: 1778660400}),
    m4_3.threadTs = '1778659200.000100',
    m4_3.permalink = 'https://example.slack.com/archives/C09ABCDEF12/p1778660400000300'
MERGE (t4)-[:HAS_MESSAGE]->(m4_3)
MERGE (jess)-[:AUTHORED]->(m4_3);

// ----- Explicit :DISCUSSED_IN edges -----
// These would normally be created by link-explicit.cypher running after ingest.
// We include them here so sample-data.cypher produces a complete demo graph
// without needing to run the linking pass separately.

MATCH (i1:Issue {identifier: 'NODES-1'})
MATCH (i5:Issue {identifier: 'NODES-5'})
MATCH (i6:Issue {identifier: 'NODES-6'})
MATCH (i3:Issue {identifier: 'NODES-3'})
MATCH (t1:Thread {channelId: 'C09ABCDEF12', ts: '1778486400.000100'})
MATCH (t2:Thread {channelId: 'C09ABCDEF12', ts: '1778572800.000100'})
MATCH (t3:Thread {channelId: 'C09ABCDEF12', ts: '1778580000.000100'})

// NODES-1 is discussed in t1 (the original problem) and t2 (cross-reference)
MERGE (i1)-[d1:DISCUSSED_IN]->(t1)
ON CREATE SET d1.confidence = 1.0,
              d1.evidence = 'explicit_mention',
              d1.createdAt = datetime()

MERGE (i1)-[d2:DISCUSSED_IN]->(t2)
ON CREATE SET d2.confidence = 1.0,
              d2.evidence = 'explicit_mention',
              d2.createdAt = datetime()

// NODES-5 in t2
MERGE (i5)-[d3:DISCUSSED_IN]->(t2)
ON CREATE SET d3.confidence = 1.0,
              d3.evidence = 'explicit_mention',
              d3.createdAt = datetime()

// NODES-6 in t2 (mentioned as a follow-up to open)
MERGE (i6)-[d4:DISCUSSED_IN]->(t2)
ON CREATE SET d4.confidence = 1.0,
              d4.evidence = 'explicit_mention',
              d4.createdAt = datetime()

// NODES-3 in t3
MERGE (i3)-[d5:DISCUSSED_IN]->(t3)
ON CREATE SET d5.confidence = 1.0,
              d5.evidence = 'explicit_mention',
              d5.createdAt = datetime()

// NOTE: t4 has no :DISCUSSED_IN edge despite being about the same problem
// as t1. The regex linking pass can't catch it because the message text
// says "the schema problem" rather than naming NODES-1 explicitly.
// In iteration 2, the semantic linking pass should create the edge with
// confidence around 0.8 and evidence 'semantic_match'.

RETURN 'sample data loaded' AS status;
