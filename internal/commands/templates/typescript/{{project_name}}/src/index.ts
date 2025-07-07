// Import types from the generated interface
// Run 'okra dev' to generate service.interface.ts from your schema
import type { GreetRequest, GreetResponse } from './service.interface';

/**
 * Implementation of the greet method from GreeterService
 * This function will be called by the OKRA runtime when the greet method is invoked
 */
export function greet(input: GreetRequest): GreetResponse {
  return {
    message: `Hello, ${input.name}!`
  };
}