// Generate the plain (executable) SDL the Cosmo Router composes for the
// neo4j subgraph.
//
//   node gen-plain-schema.cjs
//
// Reads schema.graphql (the hand-written typeDefs, single source of
// truth), builds the @neo4j/graphql schema, and writes the printed
// executable SDL to neo4j-plain-schema.graphql. router/graph.yaml
// .template points at that generated file.
//
// The output is a build artifact: gitignored, regenerated via
// `make gen-neo4j-schema` (router/Makefile), which `make compose`
// depends on. No database connection is needed.
const { Neo4jGraphQL } = require('@neo4j/graphql');
const { printSchema } = require('graphql');
const fs = require('fs').promises;
const path = require('path');

async function main() {
    const typeDefs = await fs.readFile(path.join(__dirname, 'schema.graphql'), 'utf-8');
    const neo4jSchema = new Neo4jGraphQL({ typeDefs, driver: null, debug: false });
    const schema = await neo4jSchema.getSchema();
    const out = path.join(__dirname, 'neo4j-plain-schema.graphql');
    await fs.writeFile(out, printSchema(schema) + '\n');
    console.log('wrote', out);
}

main().catch((e) => {
    console.error(e);
    process.exit(1);
});
