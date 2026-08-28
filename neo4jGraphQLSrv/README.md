# Neo4js GraphQL Library with Yoga GraphQL Server



## Install

```bash
npm init es6 --yes
npm i graphql-yoga @neo4j/graphql @neo4j/introspector neo4j-driver graphql
```


## Configuration

uses environmental variables for neo4j db configuration

```bash
export NEO4J_URI=bolt://localhost:7687
export NEO4J_USER=neo4j
export NEO4J_PASSWORD=password
export NEO4J_DATABASE=neo4j  
```

or store these in .env


On startup introspection is used to generate the graphql type defs.  If you don't want this, change the index.cjs file



## Running

