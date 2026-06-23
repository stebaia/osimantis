import '../../../../core/error/result.dart';
import '../entities/person_card.dart';

/// Contratto del repository della scheda persona (domain). Il bloc dipende solo
/// da questa astrazione.
abstract class PersonRepository {
  /// Carica la scheda completa di una persona dato il suo id.
  Future<Result<PersonCard>> getPerson(int id);
}
