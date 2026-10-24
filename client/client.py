import asyncio
import grpc
import time
import os

# Dependências geradas via: python -m grpc_tools.codegen -I../proto --python_out=. --grpc_python_out=. ../proto/gateway.proto
import gateway_pb2
import gateway_pb2_grpc

async def stream_inference(stub, provider: str, prompt: str):
    print(f"\n[CLIENT] Initializing Pipeline for {provider.upper()}...")
    
    request = gateway_pb2.InferenceRequest(
        provider=provider,
        prompt=prompt,
        temperature=0.7,
        max_tokens=256
    )
    
    try:
        metadata = [('user-agent', 'mamaa-connect-client-v1')]
        stream = stub.StreamUnifiedInference(request, metadata=metadata)
        
        for response in stream:
            print(f"[{response.model_used.upper()} | {response.timestamp}]: {response.chunk[:50]}...")
            await asyncio.sleep(0.01) # Cede o controle para o loop assíncrono
            
    except grpc.RpcError as e:
        print(f"[-] RPC Failed for {provider}: {e.details()}")

async def main():
    # Conecta ao gateway proxy em Go
    async with grpc.aio.insecure_channel('localhost:50051') as channel:
        stub = gateway_pb2_grpc.MamaaBridgeStub(channel)
        
        # Dispara chamadas simultâneas para simular processamento distribuído multi-cloud
        tasks = [
            stream_inference(stub, "google", "Explain quantum computing in one sentence."),
            stream_inference(stub, "meta", "Write a high-performance rust macro."),
            stream_inference(stub, "microsoft", "Optimize this SQL query execution plan.")
        ]
        
        await asyncio.gather(*tasks)

if __name__ == '__main__':
    print("[MAMAA-Connect] Starting Parallel Multi-Cloud Client Test Suite...")
    asyncio.run(main())
