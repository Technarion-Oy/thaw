import json
import random
import time

def generate_json_data(filename="dummy_data.json", num_rows=1000000):
    cities = ["New York", "London", "Tokyo", "Paris", "Berlin", "Sydney"]
    
    print(f"Generating {num_rows} JSON objects...")
    start_time = time.time()

    with open(filename, "w") as f:
        f.write("[\n")  # Start of JSON array
        
        for i in range(1, num_rows + 1):
            record = {
                "id": i,
                "name": f"User_{i}",
                "age": random.randint(18, 80),
                "city": random.choice(cities),
                "active": random.choice([True, False])
            }
            
            json.dump(record, f)
            
            # Add a comma for all but the last record
            if i < num_rows:
                f.write(",\n")
            else:
                f.write("\n")
                
        f.write("]")  # End of JSON array

    print(f"Done! Created '{filename}' in {time.time() - start_time:.2f} seconds.")

if __name__ == "__main__":
    generate_json_data()