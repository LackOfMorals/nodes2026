const { Neo4jGraphQL } = require('@neo4j/graphql')
const neo4j = require('neo4j-driver');
const fs = require('fs').promises;
const path = require('path');
const { createServer } = require('node:http');
const { createYoga } = require('graphql-yoga');
const { toGraphQLTypeDefs } = require('@neo4j/introspector');

const config = {
    neo4jUri: process.env.NEO4J_URI || `bolt://localhost:7687`,
    neo4jUser: process.env.NEO4J_USER || 'neo4j',
    neo4jPassword: process.env.NEO4J_PASSWORD || 'password',
    neo4jDatabase: process.env.NEO4J_DATABASE || 'neo4j',
    neo4jIntrospect: process.env.NEO4J_INTROSPECT || 'false',
    neo4jSchema: process.env.NEO4J_SCHEMA || 'schema.graphql'
};




async function loadSchemaFromFile() {
    if (config.neo4jSchema) {
        return await fs.readFile(config.neo4jSchema, 'utf-8');
    }
}

async function writeSchemaToFile(filePath, contents) {
    return await fs.writeFile(filePath, contents, 'utf-8');
}


async function runSrvr() {
    console.log('🚀 Yoga server with Neo4j library');
    console.log('======================================\n');
    console.log('Neo4j URI ', config.neo4jUri);
    console.log('Neo4j user ', config.neo4jUser);
    console.log('Neo4j pwd ', config.neo4jPassword);
    console.log('Neo4j db ', config.neo4jDatabase);
    console.log('Neo4j introspect ', config.neo4jIntrospect);


    let driver;
    if (config.neo4jUri && config.neo4jPassword) {
        driver = neo4j.driver(
            config.neo4jUri,
            neo4j.auth.basic(config.neo4jUser, config.neo4jPassword)
        );
    }

    let typeDefs;
    // TypeDefs will come from introspection or a file
    if (config.neo4jIntrospect == 'true') {
            const sessionFactory = () => driver.session( { defaultAccessMode: neo4j.session.READ} );
            typeDefs = await toGraphQLTypeDefs(sessionFactory)
            // We will also write these to file in case
            // a switch to read from file is desired
            await writeSchemaToFile(config.neo4jSchema, typeDefs)
        }
    else {
           typeDefs = await loadSchemaFromFile();
        }

    if (!typeDefs) {
        throw new Error('Failed to load type defs: introspection and file both produced no schema');
    }

    
    const neo4jSchema = new Neo4jGraphQL({
        typeDefs,
        driver,
        debug: true,
    });
    
    
    // Create a Yoga instance with the neo4j schema and set the neo4j database to use
    const yoga = createYoga({
        schema: await neo4jSchema.getSchema(),
        context: async ({ request }) => ({
          driver,
          driverConfig: {
            database: config.neo4jDatabase // Set the database
          }
        })
      })

    // Pass it into a server to hook into request handlers.
    const server = createServer(yoga)
    
    // Start the server and all is well
    server.listen(4000, () => {
    console.info('Server is running on http://localhost:4000/graphql')
    })
}

runSrvr().catch((error) => {
    console.error('❌ Error:', error);
    process.exit(1);
});
