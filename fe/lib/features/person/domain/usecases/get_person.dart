import '../../../../core/error/result.dart';
import '../entities/person_card.dart';
import '../repositories/person_repository.dart';

/// Usecase: carica la scheda di una persona dato il suo id. Sottile, ma tiene il
/// bloc disaccoppiato dal repository.
class GetPerson {
  const GetPerson(this._repository);
  final PersonRepository _repository;

  Future<Result<PersonCard>> call(int id) => _repository.getPerson(id);
}
