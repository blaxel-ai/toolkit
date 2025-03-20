import fs from 'fs';
import yaml from 'yaml';

const cache: {
  [key: string]: {
    [key: string]: any
  }
} = {}


try {
  const cacheString = fs.readFileSync('.cache.yaml', 'utf8')
  const cacheData = yaml.parseAllDocuments(cacheString)
  for (const doc of cacheData) {
    const jsonDoc = doc.toJSON()
    cache[jsonDoc.kind] = cache[jsonDoc.kind] || []
    cache[jsonDoc.kind][jsonDoc.metadata.name] = jsonDoc
  }
/* eslint-disable */
} catch (error) {
}

export async function findFromCache(resource: string, name: string): Promise<any | null> {
  return cache[resource][name]
}